package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	exportapp "github.com/afonsocosta/visto/internal/application/export"
)

func (store *Store) Export(ctx context.Context, userID string) (exportapp.Data, error) {
	data := exportapp.Data{Library: []exportapp.LibraryItem{}, Plays: []exportapp.Play{}}
	libraryRows, err := store.DB.QueryContext(ctx, `SELECT um.media_id,m.title,m.media_type,um.status,um.rating FROM user_media um JOIN media m ON m.id=um.media_id WHERE um.user_id=? ORDER BY um.updated_at DESC`, userID)
	if err != nil {
		return data, fmt.Errorf("export library: %w", err)
	}
	defer libraryRows.Close()
	for libraryRows.Next() {
		var item exportapp.LibraryItem
		var rating sql.NullInt64
		if err := libraryRows.Scan(&item.MediaID, &item.Title, &item.Type, &item.Status, &rating); err != nil {
			return data, fmt.Errorf("scan export library item: %w", err)
		}
		if rating.Valid {
			value := int(rating.Int64)
			item.Rating = &value
		}
		data.Library = append(data.Library, item)
	}
	if err := libraryRows.Err(); err != nil {
		return data, fmt.Errorf("iterate export library: %w", err)
	}
	playRows, err := store.DB.QueryContext(ctx, `SELECT id,media_id,episode_id,watched_at FROM plays WHERE user_id=? ORDER BY watched_at DESC,id DESC`, userID)
	if err != nil {
		return data, fmt.Errorf("export plays: %w", err)
	}
	defer playRows.Close()
	for playRows.Next() {
		var play exportapp.Play
		var mediaID, episodeID sql.NullString
		var watchedAt string
		if err := playRows.Scan(&play.ID, &mediaID, &episodeID, &watchedAt); err != nil {
			return data, fmt.Errorf("scan export play: %w", err)
		}
		if mediaID.Valid {
			value := mediaID.String
			play.MediaID = &value
		}
		if episodeID.Valid {
			value := episodeID.String
			play.EpisodeID = &value
		}
		play.WatchedAt, err = time.Parse(time.RFC3339Nano, watchedAt)
		if err != nil {
			return data, fmt.Errorf("parse export play time: %w", err)
		}
		data.Plays = append(data.Plays, play)
	}
	if err := playRows.Err(); err != nil {
		return data, fmt.Errorf("iterate export plays: %w", err)
	}
	return data, nil
}

var _ exportapp.Repository = (*Store)(nil)
