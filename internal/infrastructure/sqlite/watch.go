package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
			if airDate.Valid && strings.TrimSpace(airDate.String) != "" {
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

func (store *Store) ListShowEpisodes(ctx context.Context, userID, showID string) ([]watch.ShowEpisode, error) {
	var hasAccess bool
	if err := store.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_media WHERE user_id=? AND media_id=?)`, userID, showID).Scan(&hasAccess); err != nil {
		return nil, fmt.Errorf("check show access: %w", err)
	}
	if !hasAccess {
		return nil, watch.ErrShowNotFound
	}
	rows, err := store.DB.QueryContext(ctx, `SELECT e.id,e.show_id,e.season_number,e.episode_number,e.air_date,e.name,e.overview,e.runtime,e.still_path,
		EXISTS(SELECT 1 FROM plays p WHERE p.user_id=? AND p.episode_id=e.id)
		FROM episodes e WHERE e.show_id=? ORDER BY e.season_number,e.episode_number`, userID, showID)
	if err != nil {
		return nil, fmt.Errorf("list show episodes: %w", err)
	}
	defer rows.Close()
	entries := []watch.ShowEpisode{}
	for rows.Next() {
		var entry watch.ShowEpisode
		var airDate, overview, stillPath sql.NullString
		var runtime sql.NullInt64
		if err := rows.Scan(&entry.Episode.ID, &entry.Episode.ShowID, &entry.Episode.SeasonNumber, &entry.Episode.EpisodeNumber, &airDate, &entry.Name, &overview, &runtime, &stillPath, &entry.Watched); err != nil {
			return nil, fmt.Errorf("scan show episode: %w", err)
		}
		if overview.Valid {
			entry.Overview = overview.String
		}
		if runtime.Valid {
			entry.Runtime = int(runtime.Int64)
		}
		if stillPath.Valid {
			entry.StillPath = stillPath.String
		}
		if airDate.Valid && strings.TrimSpace(airDate.String) != "" {
			parsed, err := time.Parse(time.DateOnly, airDate.String)
			if err != nil {
				return nil, fmt.Errorf("parse show episode air date: %w", err)
			}
			entry.Episode.AirDate = &parsed
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate show episodes: %w", err)
	}
	return entries, nil
}

func (store *Store) ListShowSeasons(ctx context.Context, userID, showID string) ([]watch.Season, error) {
	var hasAccess bool
	if err := store.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_media WHERE user_id=? AND media_id=?)`, userID, showID).Scan(&hasAccess); err != nil {
		return nil, fmt.Errorf("check show access: %w", err)
	}
	if !hasAccess {
		return nil, watch.ErrShowNotFound
	}
	rows, err := store.DB.QueryContext(ctx, `SELECT id,show_id,season_number,COALESCE(name,''),COALESCE(episode_count,0),COALESCE(air_date,'') FROM seasons WHERE show_id=? ORDER BY season_number`, showID)
	if err != nil {
		return nil, fmt.Errorf("list show seasons: %w", err)
	}
	defer rows.Close()
	seasons := []watch.Season{}
	for rows.Next() {
		var season watch.Season
		if err := rows.Scan(&season.ID, &season.ShowID, &season.Number, &season.Name, &season.EpisodeCount, &season.AirDate); err != nil {
			return nil, fmt.Errorf("scan show season: %w", err)
		}
		seasons = append(seasons, season)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate show seasons: %w", err)
	}
	return seasons, nil
}

func (store *Store) ListSeasonEpisodes(ctx context.Context, userID, seasonID string) ([]watch.ShowEpisode, error) {
	var hasAccess bool
	if err := store.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM seasons s JOIN user_media um ON um.media_id=s.show_id WHERE s.id=? AND um.user_id=?)`, seasonID, userID).Scan(&hasAccess); err != nil {
		return nil, fmt.Errorf("check season access: %w", err)
	}
	if !hasAccess {
		return nil, watch.ErrShowNotFound
	}
	rows, err := store.DB.QueryContext(ctx, `SELECT e.id,e.show_id,e.season_number,e.episode_number,e.air_date,e.name,
		EXISTS(SELECT 1 FROM plays p WHERE p.user_id=? AND p.episode_id=e.id)
		FROM episodes e WHERE e.season_id=? ORDER BY e.episode_number`, userID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("list season episodes: %w", err)
	}
	defer rows.Close()
	entries := []watch.ShowEpisode{}
	for rows.Next() {
		var entry watch.ShowEpisode
		var airDate sql.NullString
		if err := rows.Scan(&entry.Episode.ID, &entry.Episode.ShowID, &entry.Episode.SeasonNumber, &entry.Episode.EpisodeNumber, &airDate, &entry.Name, &entry.Watched); err != nil {
			return nil, fmt.Errorf("scan season episode: %w", err)
		}
		if airDate.Valid && strings.TrimSpace(airDate.String) != "" {
			parsed, err := time.Parse(time.DateOnly, airDate.String)
			if err != nil {
				return nil, fmt.Errorf("parse season episode air date: %w", err)
			}
			entry.Episode.AirDate = &parsed
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate season episodes: %w", err)
	}
	return entries, nil
}

func (store *Store) Timezone(ctx context.Context, userID string) (string, error) {
	var timezone string
	err := store.DB.QueryRowContext(ctx, `SELECT timezone FROM user_settings WHERE user_id=?`, userID).Scan(&timezone)
	if err != nil {
		return "", fmt.Errorf("get user timezone: %w", err)
	}
	return timezone, nil
}

func (store *Store) ShowsNeedingCatalogRefresh(ctx context.Context, activeTTL, finishedTTL time.Duration) ([]int64, error) {
	activeCutoff := time.Now().UTC().Add(-activeTTL).Format(time.RFC3339Nano)
	finishedCutoff := time.Now().UTC().Add(-finishedTTL).Format(time.RFC3339Nano)
	rows, err := store.DB.QueryContext(ctx, `SELECT DISTINCT m.tmdb_id FROM user_media um JOIN media m ON m.id=um.media_id
		WHERE m.media_type='tv' AND ((m.status IN ('Ended','Canceled','Cancelled') AND (m.catalog_updated_at IS NULL OR m.catalog_updated_at<?)) OR
		(COALESCE(m.status,'') NOT IN ('Ended','Canceled','Cancelled') AND (m.catalog_updated_at IS NULL OR m.catalog_updated_at<?)))
		ORDER BY m.catalog_updated_at IS NOT NULL, m.catalog_updated_at`, finishedCutoff, activeCutoff)
	if err != nil {
		return nil, fmt.Errorf("list catalog refresh IDs: %w", err)
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan catalog refresh ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate catalog refresh IDs: %w", err)
	}
	return ids, nil
}

var _ watch.Repository = (*Store)(nil)
var _ interface {
	ListShowEpisodes(context.Context, string, string) ([]watch.ShowEpisode, error)
	ListShowSeasons(context.Context, string, string) ([]watch.Season, error)
	ListSeasonEpisodes(context.Context, string, string) ([]watch.ShowEpisode, error)
} = (*Store)(nil)
var _ interface {
	ShowsNeedingCatalogRefresh(context.Context, time.Duration, time.Duration) ([]int64, error)
	ImportShowMetadata(context.Context, string, domain.TVShowMetadata) error
} = (*Store)(nil)
