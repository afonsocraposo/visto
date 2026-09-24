package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
)

func (store *Store) CreatePlay(ctx context.Context, play tracking.Play) error {
	var mediaID, episodeID any
	if play.MediaID != nil {
		mediaID = *play.MediaID
	}
	if play.EpisodeID != nil {
		episodeID = *play.EpisodeID
	}
	_, err := store.DB.ExecContext(ctx, `INSERT INTO plays(id,user_id,media_id,episode_id,watched_at,source,created_at) VALUES(?,?,?,?,?,?,?)`, play.ID, play.UserID, mediaID, episodeID, play.WatchedAt.Format(time.RFC3339Nano), play.Source, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create play: %w", err)
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
