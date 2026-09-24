package sqlite

import (
	"context"
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

func (store *Store) UpdatePlay(ctx context.Context, userID, playID string, watchedAt time.Time) error {
	result, err := store.DB.ExecContext(ctx, `UPDATE plays SET watched_at=? WHERE id=? AND user_id=?`, watchedAt.Format(time.RFC3339Nano), playID, userID)
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
	return nil
}

func (store *Store) DeletePlay(ctx context.Context, userID, playID string) error {
	result, err := store.DB.ExecContext(ctx, `DELETE FROM plays WHERE id=? AND user_id=?`, playID, userID)
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
	return nil
}

var _ tracking.Repository = (*Store)(nil)
