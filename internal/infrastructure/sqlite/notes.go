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

func (s *Store) GetEpisodeNote(ctx context.Context, userID, episodeID string) (string, error) {
	var note string
	err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(n.note,'') FROM episodes e LEFT JOIN episode_notes n ON n.episode_id=e.id AND n.user_id=? WHERE e.id=?`, userID, episodeID).Scan(&note)
	if errors.Is(err, sql.ErrNoRows) {
		return "", library.ErrMediaNotFound
	}
	return note, err
}

func (s *Store) SetEpisodeNote(ctx context.Context, userID, episodeID, note string) error {
	if note == "" {
		if _, err := s.GetEpisodeNote(ctx, userID, episodeID); err != nil {
			return err
		}
		// Deleting a note must not remove the episode or its watch history.
		_, err := s.DB.ExecContext(ctx, `DELETE FROM episode_notes WHERE user_id=? AND episode_id=?`, userID, episodeID)
		return err
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO episode_notes(user_id,episode_id,note,updated_at)
		SELECT ?,id,?,? FROM episodes WHERE id=?
		ON CONFLICT(user_id,episode_id) DO UPDATE SET note=excluded.note,updated_at=excluded.updated_at`, userID, note, time.Now().UTC().Format(time.RFC3339Nano), episodeID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return library.ErrMediaNotFound
	}
	return err
}
