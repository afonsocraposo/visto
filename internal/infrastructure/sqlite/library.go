package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/afonsocosta/visto/internal/application/library"
	"time"
)

func (s *Store) UpsertMedia(ctx context.Context, media library.Media) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO media(id,media_type,tmdb_id,title,original_title,overview,release_date,poster_path,original_language,metadata_updated_at,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(media_type,tmdb_id) DO UPDATE SET title=excluded.title,original_title=excluded.original_title,overview=excluded.overview,release_date=excluded.release_date,poster_path=excluded.poster_path,original_language=excluded.original_language,metadata_updated_at=excluded.metadata_updated_at`,
		media.ID, media.Type, media.TMDBID, media.Title, media.OriginalTitle, media.Overview, media.ReleaseDate, media.PosterPath, media.OriginalLanguage, now, now)
	if err != nil {
		return fmt.Errorf("upsert media: %w", err)
	}
	return nil
}

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

func (s *Store) ListItems(ctx context.Context, userID string) ([]library.Entry, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT um.user_id,um.media_id,um.status,um.rating,um.added_at,um.updated_at,m.media_type,m.tmdb_id,m.title,m.original_title,m.overview,m.release_date,m.poster_path,m.original_language
		FROM user_media um JOIN media m ON m.id=um.media_id WHERE um.user_id=? ORDER BY um.updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list library items: %w", err)
	}
	defer rows.Close()
	entries := []library.Entry{}
	for rows.Next() {
		var entry library.Entry
		var rating sql.NullInt64
		var addedAt, updatedAt string
		if err := rows.Scan(&entry.Item.UserID, &entry.Item.MediaID, &entry.Item.Status, &rating, &addedAt, &updatedAt, &entry.Media.Type, &entry.Media.TMDBID, &entry.Media.Title, &entry.Media.OriginalTitle, &entry.Media.Overview, &entry.Media.ReleaseDate, &entry.Media.PosterPath, &entry.Media.OriginalLanguage); err != nil {
			return nil, fmt.Errorf("scan library item: %w", err)
		}
		entry.Media.ID = entry.Item.MediaID
		if rating.Valid {
			value := int(rating.Int64)
			entry.Item.Rating = &value
		}
		entry.Item.AddedAt, err = time.Parse(time.RFC3339Nano, addedAt)
		if err != nil {
			return nil, fmt.Errorf("parse library added time: %w", err)
		}
		entry.Item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse library updated time: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate library items: %w", err)
	}
	return entries, nil
}

var _ library.Repository = (*Store)(nil)
var _ interface {
	UpsertMedia(context.Context, library.Media) error
	ListItems(context.Context, string) ([]library.Entry, error)
} = (*Store)(nil)
