package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
)

func (s *Store) UpsertMedia(ctx context.Context, media library.Media) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO media(id,media_type,tmdb_id,title,original_title,overview,release_date,poster_path,original_language,status,metadata_updated_at,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(media_type,tmdb_id) DO UPDATE SET title=excluded.title,original_title=excluded.original_title,overview=excluded.overview,release_date=excluded.release_date,poster_path=excluded.poster_path,original_language=excluded.original_language,status=COALESCE(NULLIF(excluded.status,''),media.status),metadata_updated_at=excluded.metadata_updated_at`,
		media.ID, media.Type, media.TMDBID, media.Title, media.OriginalTitle, media.Overview, media.ReleaseDate, media.PosterPath, media.OriginalLanguage, media.Status, now, now)
	if err != nil {
		return fmt.Errorf("upsert media: %w", err)
	}
	return nil
}

func (s *Store) ImportShowMetadata(ctx context.Context, showID string, show domain.TVShowMetadata) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin show metadata import: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE media SET title=?,original_title=?,overview=?,release_date=?,poster_path=?,original_language=?,status=?,metadata_updated_at=?,catalog_updated_at=? WHERE id=? AND media_type='tv' AND tmdb_id=?`, show.Name, show.Name, show.Overview, show.FirstAirDate, show.PosterPath, show.OriginalLanguage, show.Status, now, now, showID, show.TMDBID); err != nil {
		return fmt.Errorf("update show metadata: %w", err)
	}
	for _, season := range show.Seasons {
		seasonID := fmt.Sprintf("%s:season:%d", showID, season.Number)
		var seasonTMDB any
		if season.TMDBID > 0 {
			seasonTMDB = season.TMDBID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO seasons(id,show_id,tmdb_id,season_number,name,overview,poster_path,air_date,episode_count) VALUES(?,?,?,?,?,?,?,?,?)
			ON CONFLICT(show_id,season_number) DO UPDATE SET tmdb_id=excluded.tmdb_id,name=excluded.name,overview=excluded.overview,poster_path=excluded.poster_path,air_date=excluded.air_date,episode_count=excluded.episode_count`, seasonID, showID, seasonTMDB, season.Number, season.Name, season.Overview, season.PosterPath, season.AirDate, len(season.Episodes)); err != nil {
			return fmt.Errorf("upsert season %d: %w", season.Number, err)
		}
		for _, episode := range season.Episodes {
			episodeID := fmt.Sprintf("%s:episode:%d", showID, episode.TMDBID)
			var episodeTMDB any
			if episode.TMDBID > 0 {
				episodeTMDB = episode.TMDBID
			} else {
				episodeID = fmt.Sprintf("%s:episode:%d:%d", showID, episode.SeasonNumber, episode.EpisodeNumber)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO episodes(id,show_id,season_id,tmdb_id,season_number,episode_number,name,overview,air_date,runtime,still_path,metadata_updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
				ON CONFLICT(show_id,season_number,episode_number) DO UPDATE SET season_id=excluded.season_id,tmdb_id=excluded.tmdb_id,name=excluded.name,overview=excluded.overview,air_date=excluded.air_date,runtime=excluded.runtime,still_path=excluded.still_path,metadata_updated_at=excluded.metadata_updated_at`, episodeID, showID, seasonID, episodeTMDB, episode.SeasonNumber, episode.EpisodeNumber, episode.Name, episode.Overview, episode.AirDate, episode.Runtime, episode.StillPath, now); err != nil {
				return fmt.Errorf("upsert S%02dE%02d: %w", episode.SeasonNumber, episode.EpisodeNumber, err)
			}
		}
	}
	return tx.Commit()
}

func (s *Store) ShowMetadataNeedsRefresh(ctx context.Context, showID string, ttl time.Duration) (bool, error) {
	var updated sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT catalog_updated_at FROM media WHERE id=? AND media_type='tv'`, showID).Scan(&updated)
	if err != nil {
		return false, fmt.Errorf("get show metadata age: %w", err)
	}
	if !updated.Valid {
		return true, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, updated.String)
	if err != nil {
		return true, nil
	}
	return time.Since(parsed) >= ttl, nil
}

func (s *Store) UpsertItem(ctx context.Context, item library.Item) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin library transaction: %w", err)
	}
	defer tx.Rollback()
	var previousRating sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT rating FROM user_media WHERE user_id=? AND media_id=?`, item.UserID, item.MediaID).Scan(&previousRating)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get existing library item: %w", err)
	}
	var rating any
	if item.Rating != nil {
		rating = *item.Rating
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(user_id,media_id) DO UPDATE SET status=excluded.status,rating=excluded.rating,updated_at=excluded.updated_at`, item.UserID+":"+item.MediaID, item.UserID, item.MediaID, item.Status, rating, item.AddedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert library item: %w", err)
	}
	if item.Rating != nil && (!previousRating.Valid || int(previousRating.Int64) != *item.Rating) {
		var visibility string
		if err := tx.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, item.UserID).Scan(&visibility); err != nil {
			return fmt.Errorf("get activity visibility: %w", err)
		}
		if visibility == "instance" {
			eventID := fmt.Sprintf("rating:%s:%s:%d", item.UserID, item.MediaID, item.UpdatedAt.UnixNano())
			if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(id,user_id,kind,media_id,rating,occurred_at,created_at) VALUES(?,?,?,?,?,?,?)`, eventID, item.UserID, "rating", item.MediaID, *item.Rating, item.UpdatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
				return fmt.Errorf("create rating activity event: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit library item: %w", err)
	}
	return nil
}

func (s *Store) ListItems(ctx context.Context, userID string) ([]library.Entry, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT um.user_id,um.media_id,um.status,um.rating,um.added_at,um.updated_at,m.media_type,m.tmdb_id,m.title,COALESCE(m.original_title,''),COALESCE(m.overview,''),COALESCE(m.release_date,''),COALESCE(m.poster_path,''),COALESCE(m.original_language,''),COALESCE(m.status,''),
		((m.media_type='movie' AND EXISTS(SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.media_id=um.media_id)) OR
		 (m.media_type='tv' AND COALESCE(m.status,'') IN ('Ended','Canceled','Cancelled') AND EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0) AND NOT EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0 AND (e.air_date IS NULL OR e.air_date<=date('now')) AND NOT EXISTS(SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.episode_id=e.id))))
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
		if err := rows.Scan(&entry.Item.UserID, &entry.Item.MediaID, &entry.Item.Status, &rating, &addedAt, &updatedAt, &entry.Media.Type, &entry.Media.TMDBID, &entry.Media.Title, &entry.Media.OriginalTitle, &entry.Media.Overview, &entry.Media.ReleaseDate, &entry.Media.PosterPath, &entry.Media.OriginalLanguage, &entry.Media.Status, &entry.Completed); err != nil {
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

func (s *Store) GetMediaByTMDBID(ctx context.Context, userID string, mediaType domain.MediaType, tmdbID int64) (library.Entry, error) {
	var entry library.Entry
	var rating sql.NullInt64
	var addedAt, updatedAt string
	err := s.DB.QueryRowContext(ctx, `SELECT um.user_id,um.media_id,um.status,um.rating,um.added_at,um.updated_at,
		m.media_type,m.tmdb_id,m.title,COALESCE(m.original_title,''),COALESCE(m.overview,''),COALESCE(m.release_date,''),COALESCE(m.poster_path,''),COALESCE(m.original_language,''),COALESCE(m.status,''),
		((m.media_type='movie' AND EXISTS(SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.media_id=um.media_id)) OR
		 (m.media_type='tv' AND COALESCE(m.status,'') IN ('Ended','Canceled','Cancelled') AND EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0) AND NOT EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0 AND (e.air_date IS NULL OR e.air_date<=date('now')) AND NOT EXISTS(SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.episode_id=e.id))))
		FROM user_media um JOIN media m ON m.id=um.media_id
		WHERE um.user_id=? AND m.media_type=? AND m.tmdb_id=?`, userID, mediaType, tmdbID).Scan(
		&entry.Item.UserID, &entry.Item.MediaID, &entry.Item.Status, &rating, &addedAt, &updatedAt,
		&entry.Media.Type, &entry.Media.TMDBID, &entry.Media.Title, &entry.Media.OriginalTitle,
		&entry.Media.Overview, &entry.Media.ReleaseDate, &entry.Media.PosterPath, &entry.Media.OriginalLanguage, &entry.Media.Status, &entry.Completed,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return library.Entry{}, library.ErrMediaNotFound
	}
	if err != nil {
		return library.Entry{}, fmt.Errorf("get library media by TMDB ID: %w", err)
	}
	entry.Media.ID = entry.Item.MediaID
	if rating.Valid {
		value := int(rating.Int64)
		entry.Item.Rating = &value
	}
	entry.Item.AddedAt, err = time.Parse(time.RFC3339Nano, addedAt)
	if err != nil {
		return library.Entry{}, fmt.Errorf("parse library added time: %w", err)
	}
	entry.Item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return library.Entry{}, fmt.Errorf("parse library updated time: %w", err)
	}
	return entry, nil
}

var _ library.Repository = (*Store)(nil)
var _ interface {
	UpsertMedia(context.Context, library.Media) error
	ListItems(context.Context, string) ([]library.Entry, error)
	GetMediaByTMDBID(context.Context, string, domain.MediaType, int64) (library.Entry, error)
} = (*Store)(nil)
