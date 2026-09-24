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

var _ tracking.Repository = (*Store)(nil)
