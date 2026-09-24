package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
)

func (store *Store) WatchingShows(ctx context.Context, userID string) ([]watch.Show, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT m.id,m.title,COALESCE(m.poster_path,'') FROM user_media um JOIN media m ON m.id=um.media_id WHERE um.user_id=? AND um.status='watching' AND m.media_type='tv' ORDER BY um.updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list watching shows: %w", err)
	}
	shows := []watch.Show{}
	for rows.Next() {
		var show watch.Show
		if err := rows.Scan(&show.ID, &show.Title, &show.PosterPath); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan watching show: %w", err)
		}
		show.Status = domain.WatchingStatus
		shows = append(shows, show)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate watching shows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range shows {
		show := &shows[index]
		episodeRows, err := store.DB.QueryContext(ctx, `SELECT id,season_number,episode_number,air_date FROM episodes WHERE show_id=? ORDER BY season_number,episode_number`, show.ID)
		if err != nil {
			return nil, fmt.Errorf("list episodes for %s: %w", show.ID, err)
		}
		for episodeRows.Next() {
			var episode domain.Episode
			var airDate sql.NullString
			if err := episodeRows.Scan(&episode.ID, &episode.SeasonNumber, &episode.EpisodeNumber, &airDate); err != nil {
				episodeRows.Close()
				return nil, fmt.Errorf("scan episode: %w", err)
			}
			episode.ShowID = show.ID
			if airDate.Valid {
				parsed, err := time.Parse("2006-01-02", airDate.String)
				if err != nil {
					episodeRows.Close()
					return nil, fmt.Errorf("parse episode air date: %w", err)
				}
				episode.AirDate = &parsed
			}
			show.Episodes = append(show.Episodes, episode)
		}
		if err := episodeRows.Err(); err != nil {
			episodeRows.Close()
			return nil, fmt.Errorf("iterate episodes: %w", err)
		}
		episodeRows.Close()
		playRows, err := store.DB.QueryContext(ctx, `SELECT id,episode_id,watched_at FROM plays WHERE user_id=? AND episode_id IN (SELECT id FROM episodes WHERE show_id=?)`, userID, show.ID)
		if err != nil {
			return nil, fmt.Errorf("list plays for %s: %w", show.ID, err)
		}
		for playRows.Next() {
			var play domain.EpisodePlay
			var watched string
			play.UserID = userID
			if err := playRows.Scan(&play.ID, &play.EpisodeID, &watched); err != nil {
				playRows.Close()
				return nil, fmt.Errorf("scan episode play: %w", err)
			}
			play.WatchedAt, err = time.Parse(time.RFC3339Nano, watched)
			if err != nil {
				playRows.Close()
				return nil, fmt.Errorf("parse watched time: %w", err)
			}
			show.Plays = append(show.Plays, play)
		}
		if err := playRows.Err(); err != nil {
			playRows.Close()
			return nil, fmt.Errorf("iterate episode plays: %w", err)
		}
		playRows.Close()
	}
	return shows, nil
}

func (store *Store) Timezone(ctx context.Context, userID string) (string, error) {
	var timezone string
	err := store.DB.QueryRowContext(ctx, `SELECT timezone FROM user_settings WHERE user_id=?`, userID).Scan(&timezone)
	if err != nil {
		return "", fmt.Errorf("get user timezone: %w", err)
	}
	return timezone, nil
}

func (store *Store) ShowsNeedingMetadataRefresh(ctx context.Context, userID string, ttl time.Duration) ([]int64, error) {
	cutoff := time.Now().UTC().Add(-ttl).Format(time.RFC3339Nano)
	rows, err := store.DB.QueryContext(ctx, `SELECT m.tmdb_id FROM user_media um JOIN media m ON m.id=um.media_id WHERE um.user_id=? AND um.status='watching' AND m.media_type='tv' AND (m.catalog_updated_at IS NULL OR m.catalog_updated_at<?) ORDER BY um.updated_at DESC`, userID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("list shows needing metadata refresh: %w", err)
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan show refresh ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate show refresh IDs: %w", err)
	}
	return ids, nil
}

var _ watch.Repository = (*Store)(nil)
var _ interface {
	ShowsNeedingMetadataRefresh(context.Context, string, time.Duration) ([]int64, error)
	ImportShowMetadata(context.Context, string, domain.TVShowMetadata) error
} = (*Store)(nil)
