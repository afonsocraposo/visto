package sqlite

import (
	"context"
	"fmt"
	"github.com/afonsocosta/visto/internal/application/library"
	"time"
)

func (s *Store) UpsertItem(ctx context.Context, item library.Item) error {
	var rating any
	if item.Rating != nil {
		rating = *item.Rating
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(user_id,media_id) DO UPDATE SET status=excluded.status,rating=excluded.rating,updated_at=excluded.updated_at`, item.UserID+":"+item.MediaID, item.UserID, item.MediaID, item.Status, rating, item.AddedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert library item: %w", err)
	}
	return nil
}

var _ library.Repository = (*Store)(nil)
