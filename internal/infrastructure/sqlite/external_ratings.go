package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/afonsocosta/visto/internal/application/ratings"
	"github.com/afonsocosta/visto/internal/domain"
)

func (s *Store) GetExternalRatings(ctx context.Context, mediaType domain.MediaType, tmdbID int64) (ratings.Scores, bool, error) {
	var raw, stamp string
	var scores ratings.Scores
	err := s.DB.QueryRowContext(ctx, `SELECT data,fetched_at FROM external_ratings WHERE media_type=? AND tmdb_id=?`, mediaType, tmdbID).Scan(&raw, &stamp)
	if errors.Is(err, sql.ErrNoRows) {
		return scores, false, nil
	}
	if err != nil {
		return scores, false, err
	}
	if err := json.Unmarshal([]byte(raw), &scores); err != nil {
		return scores, false, err
	}
	scores.FetchedAt, err = time.Parse(time.RFC3339Nano, stamp)
	return scores, true, err
}

func (s *Store) SaveExternalRatings(ctx context.Context, mediaType domain.MediaType, tmdbID int64, scores ratings.Scores) error {
	raw, err := json.Marshal(scores)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO external_ratings(media_type,tmdb_id,data,fetched_at) VALUES(?,?,?,?) ON CONFLICT(media_type,tmdb_id) DO UPDATE SET data=excluded.data,fetched_at=excluded.fetched_at`, mediaType, tmdbID, string(raw), scores.FetchedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ExternalRatingsDue(ctx context.Context, before time.Time, limit int) ([]ratings.Title, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT media_type,tmdb_id FROM external_ratings WHERE fetched_at<=? ORDER BY fetched_at,media_type,tmdb_id LIMIT ?`, before.Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	titles := []ratings.Title{}
	for rows.Next() {
		var title ratings.Title
		if err := rows.Scan(&title.Type, &title.TMDBID); err != nil {
			return nil, err
		}
		titles = append(titles, title)
	}
	return titles, rows.Err()
}
