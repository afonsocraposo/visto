package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
)

func (store *Store) GetEpisodeRating(ctx context.Context, userID, episodeID string) (tracking.EpisodeRating, error) {
	var result tracking.EpisodeRating
	result.EpisodeID = episodeID
	var rating sql.NullInt64
	var updatedAt string
	err := store.DB.QueryRowContext(ctx, `SELECT rating,updated_at FROM episode_ratings WHERE user_id=? AND episode_id=?`, userID, episodeID).Scan(&rating, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return tracking.EpisodeRating{}, fmt.Errorf("get episode rating: %w", err)
	}
	if rating.Valid {
		value := int(rating.Int64)
		result.Rating = &value
	}
	result.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return tracking.EpisodeRating{}, fmt.Errorf("parse episode rating time: %w", err)
	}
	return result, nil
}

func (store *Store) SaveEpisodeRating(ctx context.Context, userID, episodeID string, rating *int) (tracking.EpisodeRating, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return tracking.EpisodeRating{}, fmt.Errorf("begin episode rating: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM episodes WHERE id=?`, episodeID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tracking.EpisodeRating{}, fmt.Errorf("episode not found")
		}
		return tracking.EpisodeRating{}, fmt.Errorf("find episode: %w", err)
	}
	var previous sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT rating FROM episode_ratings WHERE user_id=? AND episode_id=?`, userID, episodeID).Scan(&previous); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return tracking.EpisodeRating{}, fmt.Errorf("read previous episode rating: %w", err)
	}
	now := time.Now().UTC()
	if rating == nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM episode_ratings WHERE user_id=? AND episode_id=?`, userID, episodeID); err != nil {
			return tracking.EpisodeRating{}, fmt.Errorf("clear episode rating: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx, `INSERT INTO episode_ratings(user_id,episode_id,rating,created_at,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(user_id,episode_id) DO UPDATE SET rating=excluded.rating,updated_at=excluded.updated_at`, userID, episodeID, *rating, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return tracking.EpisodeRating{}, fmt.Errorf("save episode rating: %w", err)
	}
	if rating != nil && (!previous.Valid || int(previous.Int64) != *rating) {
		var visibility string
		if err := tx.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, userID).Scan(&visibility); err != nil {
			return tracking.EpisodeRating{}, fmt.Errorf("get activity visibility: %w", err)
		}
		if visibility == "instance" {
			eventID := fmt.Sprintf("episode-rating:%s:%s:%d", userID, episodeID, now.UnixNano())
			if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(id,user_id,kind,episode_id,rating,occurred_at,created_at) VALUES(?,?,?,?,?,?,?)`, eventID, userID, "rating", episodeID, *rating, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
				return tracking.EpisodeRating{}, fmt.Errorf("create episode rating activity: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return tracking.EpisodeRating{}, fmt.Errorf("commit episode rating: %w", err)
	}
	result := tracking.EpisodeRating{EpisodeID: episodeID, UpdatedAt: now}
	if rating != nil {
		value := *rating
		result.Rating = &value
	}
	return result, nil
}
