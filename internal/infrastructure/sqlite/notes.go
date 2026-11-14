package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
)

func (s *Store) GetNote(ctx context.Context, userID, mediaID string) (string, error) {
	var note string
	err := s.DB.QueryRowContext(ctx, `SELECT note FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID).Scan(&note)
	if errors.Is(err, sql.ErrNoRows) {
		return "", library.ErrMediaNotFound
	}
	return note, err
}

func (s *Store) SetNote(ctx context.Context, userID, mediaID, note string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE user_media SET note=?,updated_at=? WHERE user_id=? AND media_id=?`, note, time.Now().UTC().Format(time.RFC3339Nano), userID, mediaID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return library.ErrMediaNotFound
	}
	return err
}
