package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
)

func (store *Store) CreatePlay(ctx context.Context, play tracking.Play) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin play transaction: %w", err)
	}
	defer tx.Rollback()
	var mediaID, episodeID any
	if play.MediaID != nil {
		mediaID = *play.MediaID
	}
	if play.EpisodeID != nil {
		episodeID = *play.EpisodeID
	}
	var existingPlays int
	if play.MediaID != nil {
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND media_id=?`, play.UserID, *play.MediaID).Scan(&existingPlays)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND episode_id=?`, play.UserID, *play.EpisodeID).Scan(&existingPlays)
	}
	if err != nil {
		return fmt.Errorf("check prior plays: %w", err)
	}
	createdAt := time.Now().UTC()
	trackedMediaID := ""
	if play.MediaID != nil {
		trackedMediaID = *play.MediaID
	} else if err := tx.QueryRowContext(ctx, `SELECT show_id FROM episodes WHERE id=?`, *play.EpisodeID).Scan(&trackedMediaID); err != nil {
		return fmt.Errorf("find show for episode play: %w", err)
	}
	if err := ensureWatchingRelationship(ctx, tx, play.UserID, trackedMediaID, createdAt); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO plays(id,user_id,media_id,episode_id,watched_at,source,created_at) VALUES(?,?,?,?,?,?,?)`, play.ID, play.UserID, mediaID, episodeID, play.WatchedAt.Format(time.RFC3339Nano), play.Source, createdAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create play: %w", err)
	}
	var visibility string
	if err := tx.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, play.UserID).Scan(&visibility); err != nil {
		return fmt.Errorf("get activity visibility: %w", err)
	}
	if visibility == "instance" {
		kind := "watch"
		if existingPlays > 0 {
			kind = "rewatch"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO activity_events(id,user_id,kind,play_id,media_id,episode_id,occurred_at,created_at) VALUES(?,?,?,?,?,?,?,?)`, "activity:"+play.ID, play.UserID, kind, play.ID, mediaID, episodeID, play.WatchedAt.Format(time.RFC3339Nano), createdAt.Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("create activity event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit play: %w", err)
	}
	return nil
}

func (store *Store) CreateBulkPlays(ctx context.Context, plays []tracking.Play) error {
	if len(plays) == 0 {
		return fmt.Errorf("bulk play list is empty")
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bulk play transaction: %w", err)
	}
	defer tx.Rollback()
	var showID string
	for index, play := range plays {
		if play.EpisodeID == nil {
			return fmt.Errorf("bulk plays must reference episodes")
		}
		var currentShow string
		if err := tx.QueryRowContext(ctx, `SELECT show_id FROM episodes WHERE id=?`, *play.EpisodeID).Scan(&currentShow); err != nil {
			return fmt.Errorf("find episode for bulk play: %w", err)
		}
		if index == 0 {
			showID = currentShow
		} else if currentShow != showID {
			return fmt.Errorf("bulk episodes must belong to one show")
		}
	}
	createdAtTime := time.Now().UTC()
	createdAt := createdAtTime.Format(time.RFC3339Nano)
	if err := ensureWatchingRelationship(ctx, tx, plays[0].UserID, showID, createdAtTime); err != nil {
		return err
	}
	for _, play := range plays {
		if _, err := tx.ExecContext(ctx, `INSERT INTO plays(id,user_id,episode_id,watched_at,source,created_at) VALUES(?,?,?,?,?,?)`, play.ID, play.UserID, *play.EpisodeID, play.WatchedAt.Format(time.RFC3339Nano), play.Source, createdAt); err != nil {
			return fmt.Errorf("create bulk play: %w", err)
		}
	}
	var visibility string
	if err := tx.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, plays[0].UserID).Scan(&visibility); err != nil {
		return fmt.Errorf("get activity visibility: %w", err)
	}
	if visibility == "instance" {
		playIDs := make([]string, 0, len(plays))
		for _, play := range plays {
			playIDs = append(playIDs, play.ID)
		}
		detailJSON, _ := json.Marshal(struct {
			Count   int      `json:"count"`
			PlayIDs []string `json:"play_ids"`
		}{Count: len(plays), PlayIDs: playIDs})
		detail := string(detailJSON)
		eventID := "bulk:" + plays[0].ID
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(id,user_id,kind,media_id,detail_json,occurred_at,created_at) VALUES(?,?,?,?,?,?,?)`, eventID, plays[0].UserID, "bulk_watch", showID, detail, plays[0].WatchedAt.Format(time.RFC3339Nano), createdAt); err != nil {
			return fmt.Errorf("create bulk activity event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bulk plays: %w", err)
	}
	return nil
}

func ensureWatchingRelationship(ctx context.Context, tx *sql.Tx, userID, mediaID string, createdAt time.Time) error {
	timestamp := createdAt.Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at)
		VALUES(?,?,?,'watching',?,?) ON CONFLICT(user_id,media_id) DO NOTHING`, userID+":"+mediaID, userID, mediaID, timestamp, timestamp)
	if err != nil {
		return fmt.Errorf("ensure library relationship for tracked media: %w", err)
	}
	return nil
}

func (store *Store) UpdatePlay(ctx context.Context, userID, playID string, watchedAt time.Time) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin play correction: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE plays SET watched_at=? WHERE id=? AND user_id=?`, watchedAt.Format(time.RFC3339Nano), playID, userID)
	if err != nil {
		return fmt.Errorf("update play: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect updated play: %w", err)
	}
	if changed == 0 {
		return tracking.ErrPlayNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM activity_events WHERE play_id=? OR (kind='bulk_watch' AND EXISTS(SELECT 1 FROM json_each(activity_events.detail_json,'$.play_ids') WHERE value=?))`, playID, playID); err != nil {
		return fmt.Errorf("remove corrected play activity: %w", err)
	}
	return tx.Commit()
}

func (store *Store) DeletePlay(ctx context.Context, userID, playID string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin play deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM plays WHERE id=? AND user_id=?`, playID, userID)
	if err != nil {
		return fmt.Errorf("delete play: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect deleted play: %w", err)
	}
	if changed == 0 {
		return tracking.ErrPlayNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM activity_events WHERE play_id=? OR (kind='bulk_watch' AND EXISTS(SELECT 1 FROM json_each(activity_events.detail_json,'$.play_ids') WHERE value=?))`, playID, playID); err != nil {
		return fmt.Errorf("remove deleted play activity: %w", err)
	}
	return tx.Commit()
}

func (store *Store) ListPlays(ctx context.Context, userID string, limit int) ([]tracking.HistoryEntry, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT p.id,p.user_id,p.media_id,p.episode_id,p.watched_at,p.source,COALESCE(movie.title,show.title,''),episode.season_number,episode.episode_number
		FROM plays p LEFT JOIN media movie ON movie.id=p.media_id LEFT JOIN episodes episode ON episode.id=p.episode_id LEFT JOIN media show ON show.id=episode.show_id
		WHERE p.user_id=? ORDER BY p.watched_at DESC,p.id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list play history: %w", err)
	}
	defer rows.Close()
	entries := []tracking.HistoryEntry{}
	for rows.Next() {
		var entry tracking.HistoryEntry
		var mediaID, episodeID sql.NullString
		var watchedAt string
		var season, number sql.NullInt64
		if err := rows.Scan(&entry.Play.ID, &entry.Play.UserID, &mediaID, &episodeID, &watchedAt, &entry.Play.Source, &entry.Title, &season, &number); err != nil {
			return nil, fmt.Errorf("scan play history: %w", err)
		}
		if mediaID.Valid {
			value := mediaID.String
			entry.Play.MediaID = &value
		}
		if episodeID.Valid {
			value := episodeID.String
			entry.Play.EpisodeID = &value
			entry.EpisodeLabel = fmt.Sprintf("S%02dE%02d", season.Int64, number.Int64)
		}
		entry.Play.WatchedAt, err = time.Parse(time.RFC3339Nano, watchedAt)
		if err != nil {
			return nil, fmt.Errorf("parse play history time: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate play history: %w", err)
	}
	return entries, nil
}

var _ tracking.Repository = (*Store)(nil)
var _ interface {
	ListPlays(context.Context, string, int) ([]tracking.HistoryEntry, error)
} = (*Store)(nil)
