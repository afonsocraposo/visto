package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
)

func (store *Store) ListPlays(ctx context.Context, userID string, limit int) ([]tracking.HistoryEntry, error) {
	return store.listPlays(ctx, userID, "", limit)
}

func (store *Store) ListMediaPlays(ctx context.Context, userID, mediaID string, limit int) ([]tracking.HistoryEntry, error) {
	return store.listPlays(ctx, userID, mediaID, limit)
}

func (store *Store) listPlays(ctx context.Context, userID, mediaID string, limit int) ([]tracking.HistoryEntry, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT p.id,p.user_id,p.media_id,p.episode_id,p.watched_at,p.source,COALESCE(movie.title,show.title,''),COALESCE(movie.tmdb_id,show.tmdb_id,0),episode.name,episode.season_number,episode.episode_number,COALESCE(episode.still_path,movie.poster_path,show.poster_path,'')
		FROM plays p LEFT JOIN media movie ON movie.id=p.media_id LEFT JOIN episodes episode ON episode.id=p.episode_id LEFT JOIN media show ON show.id=episode.show_id
		WHERE p.user_id=? AND (?='' OR p.media_id=?) ORDER BY p.watched_at DESC,p.id DESC LIMIT ?`, userID, mediaID, mediaID, limit)
	if err != nil {
		return nil, fmt.Errorf("list play history: %w", err)
	}
	defer rows.Close()
	entries := []tracking.HistoryEntry{}
	for rows.Next() {
		var entry tracking.HistoryEntry
		var mediaID, episodeID sql.NullString
		var watchedAt string
		var season, number sql.NullInt64
		var episodeName sql.NullString
		var artwork sql.NullString
		if err := rows.Scan(&entry.Play.ID, &entry.Play.UserID, &mediaID, &episodeID, &watchedAt, &entry.Play.Source, &entry.Title, &entry.TMDBID, &episodeName, &season, &number, &artwork); err != nil {
			return nil, fmt.Errorf("scan play history: %w", err)
		}
		if artwork.Valid {
			entry.ArtworkPath = artwork.String
		}
		if mediaID.Valid {
			value := mediaID.String
			entry.Play.MediaID = &value
		}
		if episodeID.Valid {
			value := episodeID.String
			entry.Play.EpisodeID = &value
			entry.EpisodeLabel = fmt.Sprintf("S%02dE%02d", season.Int64, number.Int64)
			if episodeName.Valid {
				entry.EpisodeName = episodeName.String
			}
		}
		entry.Play.WatchedAt, err = time.Parse(time.RFC3339Nano, watchedAt)
		if err != nil {
			return nil, fmt.Errorf("parse play history time: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate play history: %w", err)
	}
	return entries, nil
}
