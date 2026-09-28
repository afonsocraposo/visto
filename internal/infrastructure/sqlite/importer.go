package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/importer"
)

func (s *Store) ImportWelcomePending(ctx context.Context, userID string) (bool, error) {
	var pending bool
	err := s.DB.QueryRowContext(ctx, `SELECT import_welcome_seen_at IS NULL FROM user_settings WHERE user_id=?`, userID).Scan(&pending)
	return pending, err
}

func (s *Store) DismissImportWelcome(ctx context.Context, userID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE user_settings SET import_welcome_seen_at=COALESCE(import_welcome_seen_at,?) WHERE user_id=?`, time.Now().UTC().Format(time.RFC3339Nano), userID)
	return err
}

func (s *Store) ImportData(ctx context.Context, userID string, data importer.Data) (importer.Result, error) {
	result := importer.Result{Unsupported: data.Unsupported, Issues: append([]importer.Issue{}, data.Issues...)}
	result.Skipped = len(result.Issues)
	if data.Unsupported > 0 {
		result.Issues = append(result.Issues, importer.Issue{Item: "lists.csv", Reason: "custom lists are not supported by Visto"})
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return importer.Result{}, err
	}
	defer tx.Rollback()
	addIssue := func(item, reason string) {
		result.Skipped++
		result.Issues = append(result.Issues, importer.Issue{Item: item, Reason: reason})
	}
	newTitles := map[string]bool{}
	importedShows := map[string]bool{}
	ensureMedia := func(kind string, id int64, name, original string) (string, error) {
		mediaID := fmt.Sprintf("%s:%d", kind, id)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err := tx.ExecContext(ctx, `INSERT INTO media(id,media_type,tmdb_id,title,original_title,metadata_updated_at,created_at)
			VALUES(?,?,?,?,?,'',?) ON CONFLICT(media_type,tmdb_id) DO NOTHING`, mediaID, kind, id, name, original, now)
		return mediaID, err
	}
	ensureEpisode := func(showID string, season, episode int) (string, error) {
		seasonID := fmt.Sprintf("%s:season:%d", showID, season)
		_, err := tx.ExecContext(ctx, `INSERT INTO seasons(id,show_id,season_number,name,episode_count) VALUES(?,?,?,?,?)
			ON CONFLICT(show_id,season_number) DO UPDATE SET episode_count=MAX(COALESCE(seasons.episode_count,0),excluded.episode_count)`, seasonID, showID, season, fmt.Sprintf("Season %d", season), episode)
		if err != nil {
			return "", err
		}
		fallback := fmt.Sprintf("%s:episode:%d:%d", showID, season, episode)
		_, err = tx.ExecContext(ctx, `INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name)
			VALUES(?,?,?,?,?,?) ON CONFLICT(show_id,season_number,episode_number) DO NOTHING`, fallback, showID, seasonID, season, episode, fmt.Sprintf("Episode %d", episode))
		if err != nil {
			return "", err
		}
		var episodeID string
		err = tx.QueryRowContext(ctx, `SELECT id FROM episodes WHERE show_id=? AND season_number=? AND episode_number=?`, showID, season, episode).Scan(&episodeID)
		return episodeID, err
	}
	for _, item := range data.Titles {
		if item.TMDBID <= 0 {
			addIssue(item.Name, "no matching TMDB title")
			continue
		}
		mediaID, err := ensureMedia(item.Type, item.TMDBID, item.Name, item.OriginalTitle)
		if err != nil {
			return importer.Result{}, err
		}
		date := item.AddedAt.UTC().Format(time.RFC3339Nano)
		inserted, err := tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at)
			VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,media_id) DO NOTHING`, userID+":"+mediaID, userID, mediaID, item.Status, date, date)
		if err != nil {
			return importer.Result{}, err
		}
		if count, _ := inserted.RowsAffected(); count > 0 {
			result.Titles++
			newTitles[mediaID] = true
		}
	}
	for _, watch := range data.Watches {
		if watch.TMDBID <= 0 {
			addIssue(watch.Name, "no matching TMDB title")
			continue
		}
		kind := "tv"
		if watch.Type == "movie" {
			kind = "movie"
		}
		mediaID, err := ensureMedia(kind, watch.TMDBID, watch.Name, watch.Name)
		if err != nil {
			return importer.Result{}, err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		status := "watching"
		if kind == "movie" {
			status = "completed"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at)
			VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,media_id) DO NOTHING`, userID+":"+mediaID, userID, mediaID, status, now, now)
		if err != nil {
			return importer.Result{}, err
		}
		watched := watch.WatchedAt.UTC().Format(time.RFC3339Nano)
		var inserted sql.Result
		if kind == "movie" {
			inserted, err = tx.ExecContext(ctx, `INSERT INTO plays(user_id,media_id,watched_at,source,created_at)
				SELECT ?,?,?,'import',? WHERE NOT EXISTS(SELECT 1 FROM plays WHERE user_id=? AND media_id=?)`, userID, mediaID, watched, now, userID, mediaID)
		} else {
			episodeID, episodeErr := ensureEpisode(mediaID, watch.Season, watch.Episode)
			if episodeErr != nil {
				return importer.Result{}, episodeErr
			}
			inserted, err = tx.ExecContext(ctx, `INSERT INTO plays(user_id,episode_id,watched_at,source,created_at)
				SELECT ?,?,?,'import',? WHERE NOT EXISTS(SELECT 1 FROM plays WHERE user_id=? AND episode_id=?)`, userID, episodeID, watched, now, userID, episodeID)
			importedShows[mediaID] = true
		}
		if err != nil {
			return importer.Result{}, err
		}
		if count, _ := inserted.RowsAffected(); count > 0 {
			result.Watches++
			if kind == "movie" && newTitles[mediaID] {
				if _, err := tx.ExecContext(ctx, `UPDATE user_media SET status='completed' WHERE user_id=? AND media_id=?`, userID, mediaID); err != nil {
					return importer.Result{}, err
				}
			}
		}
	}
	for showID := range importedShows {
		complete, err := showIsComplete(ctx, tx, userID, showID)
		if err != nil {
			return importer.Result{}, err
		}
		if complete {
			if _, err := tx.ExecContext(ctx, `UPDATE user_media SET status='completed',updated_at=? WHERE user_id=? AND media_id=? AND status='watching'`, time.Now().UTC().Format(time.RFC3339Nano), userID, showID); err != nil {
				return importer.Result{}, err
			}
		}
	}
	for _, rating := range data.Ratings {
		if rating.TMDBID <= 0 {
			addIssue(rating.Name, "no matching TMDB title")
			continue
		}
		kind := rating.Type
		if kind == "episode" {
			kind = "tv"
		}
		mediaID, err := ensureMedia(kind, rating.TMDBID, rating.Name, rating.Name)
		if err != nil {
			return importer.Result{}, err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at)
			VALUES(?,?,?,'watchlist',?,?) ON CONFLICT(user_id,media_id) DO NOTHING`, userID+":"+mediaID, userID, mediaID, now, now)
		if err != nil {
			return importer.Result{}, err
		}
		if rating.Type == "episode" {
			episodeID, err := ensureEpisode(mediaID, rating.Season, rating.Episode)
			if err != nil {
				return importer.Result{}, err
			}
			inserted, err := tx.ExecContext(ctx, `INSERT INTO episode_ratings(user_id,episode_id,rating,created_at,updated_at)
				VALUES(?,?,?,?,?) ON CONFLICT(user_id,episode_id) DO NOTHING`, userID, episodeID, rating.Value, now, now)
			if err != nil {
				return importer.Result{}, err
			}
			if count, _ := inserted.RowsAffected(); count > 0 {
				result.Ratings++
			}
		} else {
			updated, err := tx.ExecContext(ctx, `UPDATE user_media SET rating=? WHERE user_id=? AND media_id=? AND rating IS NULL`, rating.Value, userID, mediaID)
			if err != nil {
				return importer.Result{}, err
			}
			if count, _ := updated.RowsAffected(); count > 0 {
				result.Ratings++
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE user_settings SET import_welcome_seen_at=COALESCE(import_welcome_seen_at,?) WHERE user_id=?`, time.Now().UTC().Format(time.RFC3339Nano), userID); err != nil {
		return importer.Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return importer.Result{}, err
	}
	return result, nil
}
