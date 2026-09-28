package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/domain"
)

// showIsComplete uses every known regular episode, including future episodes.
// A season with missing catalog episodes cannot complete a show.
func showIsComplete(ctx context.Context, tx *sql.Tx, userID, showID string) (bool, error) {
	var complete bool
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(m.status IN ('Ended','Canceled','Cancelled'),0)
		AND EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0)
		AND (SELECT COUNT(*) FROM episodes e WHERE e.show_id=m.id AND e.season_number>0) >=
			COALESCE((SELECT SUM(s.episode_count) FROM seasons s WHERE s.show_id=m.id AND s.season_number>0),0)
		AND NOT EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0
			AND NOT EXISTS(SELECT 1 FROM plays p WHERE p.user_id=? AND p.episode_id=e.id))
		FROM media m WHERE m.id=? AND m.media_type='tv'`, userID, showID).Scan(&complete)
	if err != nil {
		return false, err
	}
	return complete, nil
}

func reconcileShowStatus(ctx context.Context, tx *sql.Tx, userID, showID string) error {
	complete, err := showIsComplete(ctx, tx, userID, showID)
	if err != nil {
		return err
	}
	status := "watching"
	if complete {
		status = "completed"
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at)
			VALUES(?,?,?,'completed',?,?) ON CONFLICT(user_id,media_id) DO NOTHING`, userID+":"+showID, userID, showID, now, now); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE user_media SET status=?,updated_at=? WHERE user_id=? AND media_id=?
		AND (status='completed' OR ?='completed') AND status!=?`, status, time.Now().UTC().Format(time.RFC3339Nano), userID, showID, status, status)
	return err
}

func reconcileMediaStatus(ctx context.Context, tx *sql.Tx, userID, mediaID string) error {
	if len(mediaID) >= 3 && mediaID[:3] == "tv:" {
		return reconcileShowStatus(ctx, tx, userID, mediaID)
	}
	_, err := tx.ExecContext(ctx, `UPDATE user_media SET status='completed',updated_at=? WHERE user_id=? AND media_id=? AND status!='completed'
		AND EXISTS(SELECT 1 FROM plays WHERE user_id=? AND media_id=?)`, time.Now().UTC().Format(time.RFC3339Nano), userID, mediaID, userID, mediaID)
	return err
}

func (s *Store) CompleteMedia(ctx context.Context, userID, mediaID string, rating *int, source string) (library.Item, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return library.Item{}, err
	}
	defer tx.Rollback()
	var mediaType, showStatus string
	if err := tx.QueryRowContext(ctx, `SELECT media_type,COALESCE(status,'') FROM media WHERE id=?`, mediaID).Scan(&mediaType, &showStatus); err != nil {
		return library.Item{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if mediaType == "tv" {
		if showStatus != "Ended" && showStatus != "Canceled" && showStatus != "Cancelled" {
			return library.Item{}, fmt.Errorf("only ended or canceled shows can be completed")
		}
		var catalogCount, expectedCount int
		if err := tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM episodes WHERE show_id=? AND season_number>0),
			COALESCE((SELECT SUM(episode_count) FROM seasons WHERE show_id=? AND season_number>0),0)`, mediaID, mediaID).Scan(&catalogCount, &expectedCount); err != nil {
			return library.Item{}, err
		}
		if catalogCount == 0 || catalogCount < expectedCount {
			return library.Item{}, fmt.Errorf("show episode catalog is incomplete")
		}
	} else if mediaType != "movie" {
		return library.Item{}, fmt.Errorf("invalid media type")
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return library.Item{}, fmt.Errorf("rating must be from 1 to 5")
	}
	var previousRating sql.NullInt64
	var notifications bool
	var addedAt string
	err = tx.QueryRowContext(ctx, `SELECT rating,notifications_enabled,added_at FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID).Scan(&previousRating, &notifications, &addedAt)
	if err != nil && err != sql.ErrNoRows {
		return library.Item{}, err
	}
	if err == sql.ErrNoRows {
		notifications = true
		addedAt = now
	}
	var storedRating any
	var createdEpisodeIDs []string
	if rating != nil {
		storedRating = int64(*rating)
	} else if previousRating.Valid {
		storedRating = previousRating.Int64
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at,notifications_enabled)
		VALUES(?,?,?,'watching',?,?,?,?) ON CONFLICT(user_id,media_id) DO UPDATE SET rating=excluded.rating,updated_at=excluded.updated_at`, userID+":"+mediaID, userID, mediaID, storedRating, addedAt, now, notifications); err != nil {
		return library.Item{}, err
	}
	if mediaType == "movie" {
		result, err := tx.ExecContext(ctx, `INSERT INTO plays(user_id,media_id,watched_at,source,created_at)
			SELECT ?,?,?,?,? WHERE NOT EXISTS(SELECT 1 FROM plays WHERE user_id=? AND media_id=?)`, userID, mediaID, now, source, now, userID, mediaID)
		if err != nil {
			return library.Item{}, err
		}
		if created, _ := result.RowsAffected(); created > 0 {
			playID, _ := result.LastInsertId()
			if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(user_id,kind,play_id,media_id,occurred_at,created_at)
				SELECT ?,'watch',?,?,?,? WHERE EXISTS(SELECT 1 FROM user_settings WHERE user_id=? AND activity_visibility='instance')`, userID, playID, mediaID, now, now, userID); err != nil {
				return library.Item{}, err
			}
		}
	} else {
		rows, err := tx.QueryContext(ctx, `SELECT e.id FROM episodes e WHERE e.show_id=? AND e.season_number>0
			AND NOT EXISTS(SELECT 1 FROM plays p WHERE p.user_id=? AND p.episode_id=e.id) ORDER BY e.season_number,e.episode_number`, mediaID, userID)
		if err != nil {
			return library.Item{}, err
		}
		missing := []tracking.Play{}
		for rows.Next() {
			var episodeID string
			if err := rows.Scan(&episodeID); err != nil {
				rows.Close()
				return library.Item{}, err
			}
			missing = append(missing, tracking.Play{UserID: userID, EpisodeID: &episodeID, WatchedAt: time.Now().UTC(), Source: source})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return library.Item{}, err
		}
		rows.Close()
		if len(missing) > 0 {
			if _, err := s.createBulkPlaysInTx(ctx, tx, missing); err != nil {
				return library.Item{}, err
			}
			createdEpisodeIDs = make([]string, 0, len(missing))
			for _, play := range missing {
				createdEpisodeIDs = append(createdEpisodeIDs, *play.EpisodeID)
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE user_media SET status='completed',updated_at=? WHERE user_id=? AND media_id=?`, now, userID, mediaID); err != nil {
		return library.Item{}, err
	}
	if err = tx.Commit(); err != nil {
		return library.Item{}, err
	}
	parsedAdded, _ := time.Parse(time.RFC3339Nano, addedAt)
	parsedNow, _ := time.Parse(time.RFC3339Nano, now)
	var resultRating *int
	if storedRating != nil {
		value := int(storedRating.(int64))
		resultRating = &value
	}
	return library.Item{UserID: userID, MediaID: mediaID, Status: domain.CompletedStatus, Rating: resultRating, NotificationsEnabled: notifications, AddedAt: parsedAdded, UpdatedAt: parsedNow, CreatedEpisodeIDs: createdEpisodeIDs}, nil
}
