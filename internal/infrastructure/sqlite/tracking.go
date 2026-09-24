package sqlite

import (
	"context"
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
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
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

var _ tracking.Repository = (*Store)(nil)
