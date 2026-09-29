package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
)

type mediaExtras struct {
	Cast        []domain.TVCastMember `json:"cast"`
	Runtime     int                   `json:"runtime,omitempty"`
	Genres      []string              `json:"genres,omitempty"`
	VoteAverage float32               `json:"vote_average,omitempty"`
}

func applyMediaExtras(entry *library.Entry, raw sql.NullString) error {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	entry.DetailsReady = true
	var extra mediaExtras
	if err := json.Unmarshal([]byte(raw.String), &extra); err != nil {
		return fmt.Errorf("decode media metadata: %w", err)
	}
	entry.Cast, entry.Runtime, entry.Genres, entry.VoteAverage = extra.Cast, extra.Runtime, extra.Genres, extra.VoteAverage
	return nil
}

func (s *Store) SaveShowSummary(ctx context.Context, show domain.TVShowMetadata) error {
	raw, err := json.Marshal(mediaExtras{Cast: show.Cast})
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE media SET title=?,original_title=?,overview=?,release_date=?,poster_path=?,backdrop_path=?,original_language=?,status=?,raw_metadata=?,metadata_updated_at=? WHERE media_type='tv' AND tmdb_id=? AND (metadata_updated_at='' OR raw_metadata IS NULL)`, show.Name, show.Name, show.Overview, show.FirstAirDate, show.PosterPath, show.BackdropPath, show.OriginalLanguage, show.Status, raw, time.Now().UTC().Format(time.RFC3339Nano), show.TMDBID)
	return err
}

func (s *Store) SaveMovieMetadata(ctx context.Context, movie domain.MovieMetadata) error {
	raw, err := json.Marshal(mediaExtras{Cast: movie.Cast, Runtime: movie.Runtime, Genres: movie.Genres, VoteAverage: movie.VoteAverage})
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE media SET title=?,original_title=?,overview=?,release_date=?,poster_path=?,backdrop_path=?,original_language=?,status=?,raw_metadata=?,metadata_updated_at=? WHERE media_type='movie' AND tmdb_id=? AND (metadata_updated_at='' OR raw_metadata IS NULL)`, movie.Title, movie.OriginalTitle, movie.Overview, movie.ReleaseDate, movie.PosterPath, movie.BackdropPath, movie.OriginalLanguage, movie.Status, raw, time.Now().UTC().Format(time.RFC3339Nano), movie.TMDBID)
	return err
}

func (s *Store) RefreshMovieMetadata(ctx context.Context, movie domain.MovieMetadata) error {
	raw, err := json.Marshal(mediaExtras{Cast: movie.Cast, Runtime: movie.Runtime, Genres: movie.Genres, VoteAverage: movie.VoteAverage})
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE media SET title=?,original_title=?,overview=?,release_date=?,poster_path=?,backdrop_path=?,original_language=?,status=?,raw_metadata=?,metadata_updated_at=? WHERE media_type='movie' AND tmdb_id=?`, movie.Title, movie.OriginalTitle, movie.Overview, movie.ReleaseDate, movie.PosterPath, movie.BackdropPath, movie.OriginalLanguage, movie.Status, raw, time.Now().UTC().Format(time.RFC3339Nano), movie.TMDBID)
	return err
}

func (s *Store) UpsertMedia(ctx context.Context, media library.Media) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO media(id,media_type,tmdb_id,title,original_title,overview,release_date,poster_path,backdrop_path,original_language,status,metadata_updated_at,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(media_type,tmdb_id) DO UPDATE SET title=excluded.title,original_title=excluded.original_title,overview=excluded.overview,release_date=excluded.release_date,poster_path=excluded.poster_path,backdrop_path=excluded.backdrop_path,original_language=excluded.original_language,status=COALESCE(NULLIF(excluded.status,''),media.status),metadata_updated_at=excluded.metadata_updated_at`,
		media.ID, media.Type, media.TMDBID, media.Title, media.OriginalTitle, media.Overview, media.ReleaseDate, media.PosterPath, media.BackdropPath, media.OriginalLanguage, media.Status, now, now)
	if err != nil {
		return fmt.Errorf("upsert media: %w", err)
	}
	return nil
}

func (s *Store) MissingMetadataAfter(ctx context.Context, cursor watch.MissingMedia, limit int) ([]watch.MissingMedia, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT media_type,tmdb_id FROM media WHERE ((media_type='movie' AND metadata_updated_at='') OR (media_type='tv' AND (metadata_updated_at='' OR catalog_updated_at IS NULL))) AND (media_type>? OR (media_type=? AND tmdb_id>?)) ORDER BY media_type,tmdb_id LIMIT ?`, cursor.Type, cursor.Type, cursor.TMDBID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []watch.MissingMedia{}
	for rows.Next() {
		var item watch.MissingMedia
		if err := rows.Scan(&item.Type, &item.TMDBID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ShowMetadataMissing(ctx context.Context, tmdbID int64) (bool, error) {
	var missing bool
	err := s.DB.QueryRowContext(ctx, `SELECT metadata_updated_at='' FROM media WHERE media_type='tv' AND tmdb_id=?`, tmdbID).Scan(&missing)
	return missing, err
}

func (s *Store) ImportShowMetadata(ctx context.Context, showID string, show domain.TVShowMetadata) error {
	raw, err := json.Marshal(mediaExtras{Cast: show.Cast})
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin show metadata import: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE media SET title=?,original_title=?,overview=?,release_date=?,poster_path=?,backdrop_path=?,original_language=?,status=?,raw_metadata=?,metadata_updated_at=?,catalog_updated_at=? WHERE id=? AND media_type='tv' AND tmdb_id=?`, show.Name, show.Name, show.Overview, show.FirstAirDate, show.PosterPath, show.BackdropPath, show.OriginalLanguage, show.Status, raw, now, now, showID, show.TMDBID); err != nil {
		return fmt.Errorf("update show metadata: %w", err)
	}
	for _, season := range show.Seasons {
		seasonID := fmt.Sprintf("%s:season:%d", showID, season.Number)
		var seasonTMDB any
		if season.TMDBID > 0 {
			seasonTMDB = season.TMDBID
		}
		episodeCount := season.EpisodeCount
		if episodeCount < len(season.Episodes) {
			episodeCount = len(season.Episodes)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO seasons(id,show_id,tmdb_id,season_number,name,overview,poster_path,air_date,episode_count) VALUES(?,?,?,?,?,?,?,?,?)
			ON CONFLICT(show_id,season_number) DO UPDATE SET tmdb_id=excluded.tmdb_id,name=excluded.name,overview=excluded.overview,poster_path=excluded.poster_path,air_date=excluded.air_date,episode_count=excluded.episode_count`, seasonID, showID, seasonTMDB, season.Number, season.Name, season.Overview, season.PosterPath, season.AirDate, episodeCount); err != nil {
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
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM user_media WHERE media_id=?
		UNION SELECT p.user_id FROM plays p JOIN episodes e ON e.id=p.episode_id WHERE e.show_id=?`, showID, showID)
	if err != nil {
		return err
	}
	userIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		userIDs = append(userIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range userIDs {
		if err := reconcileShowStatus(ctx, tx, id, showID); err != nil {
			return err
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
	var previousStatus string
	err = tx.QueryRowContext(ctx, `SELECT rating,status FROM user_media WHERE user_id=? AND media_id=?`, item.UserID, item.MediaID).Scan(&previousRating, &previousStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get existing library item: %w", err)
	}
	if item.Status == domain.CompletedStatus && previousStatus != "completed" {
		return fmt.Errorf("use confirmed completion to mark this title completed")
	}
	if previousStatus == "completed" && item.Status != domain.CompletedStatus {
		return fmt.Errorf("completed titles cannot change lists while fully watched")
	}
	if item.Status == domain.WatchlistStatus {
		var watched bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM plays p LEFT JOIN episodes e ON e.id=p.episode_id
			WHERE p.user_id=? AND (p.media_id=? OR e.show_id=?))`, item.UserID, item.MediaID, item.MediaID).Scan(&watched); err != nil {
			return err
		}
		if watched {
			return fmt.Errorf("titles with watch history cannot enter Watchlist")
		}
	}
	if strings.HasPrefix(item.MediaID, "movie:") && item.Status != domain.WatchlistStatus && item.Status != domain.CompletedStatus {
		return fmt.Errorf("movies can only be in Watchlist or Completed")
	}
	var rating any
	if item.Rating != nil {
		rating = *item.Rating
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at,notifications_since)
		VALUES(?,?,?,?,?,?,?,CASE WHEN ?='watching' THEN ? ELSE NULL END)
		ON CONFLICT(user_id,media_id) DO UPDATE SET status=excluded.status,rating=excluded.rating,updated_at=excluded.updated_at,
		notifications_since=CASE WHEN excluded.status='watching' AND user_media.status!='watching' THEN excluded.updated_at ELSE user_media.notifications_since END`,
		item.UserID+":"+item.MediaID, item.UserID, item.MediaID, item.Status, rating, item.AddedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano), item.Status, item.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert library item: %w", err)
	}
	if strings.HasPrefix(item.MediaID, "tv:") {
		if err := reconcileShowStatus(ctx, tx, item.UserID, item.MediaID); err != nil {
			return err
		}
	}
	if item.Rating != nil && (!previousRating.Valid || int(previousRating.Int64) != *item.Rating) {
		var visibility string
		if err := tx.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, item.UserID).Scan(&visibility); err != nil {
			return fmt.Errorf("get activity visibility: %w", err)
		}
		if visibility == "instance" {
			createdAt := time.Now().UTC().Format(time.RFC3339Nano)
			if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(user_id,kind,media_id,rating,occurred_at,created_at) VALUES(?,?,?,?,?,?)`, item.UserID, "rating", item.MediaID, *item.Rating, item.UpdatedAt.Format(time.RFC3339Nano), createdAt); err != nil {
				return fmt.Errorf("create rating activity event: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit library item: %w", err)
	}
	return nil
}

func (s *Store) ItemStatus(ctx context.Context, userID, mediaID string) (domain.LibraryStatus, error) {
	var status domain.LibraryStatus
	err := s.DB.QueryRowContext(ctx, `SELECT status FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID).Scan(&status)
	return status, err
}

func (s *Store) RemoveWatchlistItem(ctx context.Context, userID, mediaID string) error {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM user_media WHERE user_id=? AND media_id=? AND status='watchlist'`, userID, mediaID)
	if err != nil {
		return fmt.Errorf("remove watchlist item: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check watchlist removal: %w", err)
	}
	if count == 0 {
		return library.ErrMediaNotFound
	}
	return nil
}

func (s *Store) RemoveStatusItem(ctx context.Context, userID, mediaID string, status domain.LibraryStatus) error {
	if status == domain.CompletedStatus {
		return fmt.Errorf("completed shows cannot be removed while watch history remains")
	}
	result, err := s.DB.ExecContext(ctx, `DELETE FROM user_media WHERE user_id=? AND media_id=? AND status=?`, userID, mediaID, status)
	if err != nil {
		return fmt.Errorf("remove library item: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check library removal: %w", err)
	}
	if count == 0 {
		return library.ErrMediaNotFound
	}
	return nil
}

func (s *Store) RemoveWatchingItem(ctx context.Context, userID, mediaID string) error {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM user_media WHERE user_id=? AND media_id=? AND status='watching'
		AND (media_id LIKE 'tv:%' OR NOT EXISTS (SELECT 1 FROM plays p WHERE p.user_id=? AND p.media_id=?))`, userID, mediaID, userID, mediaID)
	if err != nil {
		return fmt.Errorf("remove watching item: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check watching removal: %w", err)
	}
	if count == 0 {
		return library.ErrMediaNotFound
	}
	return nil
}

func (s *Store) ListItems(ctx context.Context, userID string) ([]library.Entry, error) {
	return s.ListItemsSorted(ctx, userID, "updated")
}

func (s *Store) ListItemsSorted(ctx context.Context, userID, sort string) ([]library.Entry, error) {
	return s.listItemsSorted(ctx, userID, sort, nil)
}

func (s *Store) listItemsSorted(ctx context.Context, userID, sort string, selected []string) ([]library.Entry, error) {
	order := "um.updated_at DESC, m.title COLLATE NOCASE, um.media_id"
	switch sort {
	case "title":
		order = "m.title COLLATE NOCASE, um.media_id"
	case "released":
		order = "COALESCE(NULLIF(m.release_date,''),'0000-00-00') DESC, m.title COLLATE NOCASE, um.media_id"
	case "updated":
	default:
		return nil, fmt.Errorf("invalid library sort")
	}
	filter := ""
	progressFilter := ""
	args := []any{userID}
	if selected != nil {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(selected)), ",")
		filter = " AND um.media_id IN (" + placeholders + ")"
		progressFilter = " AND s.show_id IN (" + placeholders + ")"
		for _, id := range selected {
			args = append(args, id)
		}
	}
	args = append(args, userID)
	if selected != nil {
		for _, id := range selected {
			args = append(args, id)
		}
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT um.user_id,um.media_id,um.status,um.rating,um.added_at,um.updated_at,um.notifications_enabled,um.season_alerts_enabled,m.media_type,m.tmdb_id,m.title,COALESCE(m.original_title,''),COALESCE(m.overview,''),COALESCE(m.release_date,''),COALESCE(m.poster_path,''),COALESCE(m.backdrop_path,''),COALESCE(m.original_language,''),COALESCE(m.status,''),m.metadata_updated_at<>'',m.raw_metadata,
		progress.watched_episodes,progress.total_episodes
		FROM user_media um JOIN media m ON m.id=um.media_id
		LEFT JOIN (
			SELECT s.show_id,(SELECT SUM(COALESCE(s2.episode_count,0)) FROM seasons s2 WHERE s2.show_id=s.show_id AND s2.season_number>0) AS total_episodes,COUNT(DISTINCT p.episode_id) AS watched_episodes
			FROM seasons s
			LEFT JOIN episodes e ON e.season_id=s.id AND e.season_number>0
			LEFT JOIN (SELECT DISTINCT episode_id FROM plays WHERE user_id=?) p ON p.episode_id=e.id
			WHERE s.season_number>0`+progressFilter+` GROUP BY s.show_id
		) progress ON progress.show_id=m.id
		WHERE um.user_id=?`+filter+` ORDER BY `+order, args...)
	if err != nil {
		return nil, fmt.Errorf("list library items: %w", err)
	}
	defer rows.Close()
	entries := []library.Entry{}
	for rows.Next() {
		var entry library.Entry
		var rating, watchedEpisodes, totalEpisodes sql.NullInt64
		var raw sql.NullString
		var addedAt, updatedAt string
		if err := rows.Scan(&entry.Item.UserID, &entry.Item.MediaID, &entry.Item.Status, &rating, &addedAt, &updatedAt, &entry.Item.NotificationsEnabled, &entry.Item.SeasonAlertsEnabled, &entry.Media.Type, &entry.Media.TMDBID, &entry.Media.Title, &entry.Media.OriginalTitle, &entry.Media.Overview, &entry.Media.ReleaseDate, &entry.Media.PosterPath, &entry.Media.BackdropPath, &entry.Media.OriginalLanguage, &entry.Media.Status, &entry.MetadataReady, &raw, &watchedEpisodes, &totalEpisodes); err != nil {
			return nil, fmt.Errorf("scan library item: %w", err)
		}
		if err := applyMediaExtras(&entry, raw); err != nil {
			return nil, err
		}
		if entry.Media.Type == domain.TVMediaType && watchedEpisodes.Valid && totalEpisodes.Valid && totalEpisodes.Int64 > 0 {
			entry.Progress = &library.ShowProgress{WatchedEpisodes: int(watchedEpisodes.Int64), TotalEpisodes: int(totalEpisodes.Int64)}
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
	var raw sql.NullString
	var addedAt, updatedAt string
	err := s.DB.QueryRowContext(ctx, `SELECT um.user_id,um.media_id,um.status,um.rating,um.added_at,um.updated_at,um.notifications_enabled,um.season_alerts_enabled,
		m.media_type,m.tmdb_id,m.title,COALESCE(m.original_title,''),COALESCE(m.overview,''),COALESCE(m.release_date,''),COALESCE(m.poster_path,''),COALESCE(m.backdrop_path,''),COALESCE(m.original_language,''),COALESCE(m.status,''),m.metadata_updated_at<>'',m.raw_metadata,m.catalog_updated_at IS NOT NULL
		FROM user_media um JOIN media m ON m.id=um.media_id
		WHERE um.user_id=? AND m.media_type=? AND m.tmdb_id=?`, userID, mediaType, tmdbID).Scan(
		&entry.Item.UserID, &entry.Item.MediaID, &entry.Item.Status, &rating, &addedAt, &updatedAt, &entry.Item.NotificationsEnabled, &entry.Item.SeasonAlertsEnabled,
		&entry.Media.Type, &entry.Media.TMDBID, &entry.Media.Title, &entry.Media.OriginalTitle,
		&entry.Media.Overview, &entry.Media.ReleaseDate, &entry.Media.PosterPath, &entry.Media.BackdropPath, &entry.Media.OriginalLanguage, &entry.Media.Status, &entry.MetadataReady, &raw, &entry.CatalogReady,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return library.Entry{}, library.ErrMediaNotFound
	}
	if err != nil {
		return library.Entry{}, fmt.Errorf("get library media by TMDB ID: %w", err)
	}
	if err := applyMediaExtras(&entry, raw); err != nil {
		return library.Entry{}, err
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

func (s *Store) SetNotificationsEnabled(ctx context.Context, userID, mediaID string, enabled bool) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin media notification preference update: %w", err)
	}
	defer tx.Rollback()
	var wasEnabled bool
	var mediaType, status string
	err = tx.QueryRowContext(ctx, `SELECT um.notifications_enabled,m.media_type,um.status FROM user_media um JOIN media m ON m.id=um.media_id
		WHERE um.user_id=? AND um.media_id=? AND ((m.media_type='tv' AND um.status='watching') OR (m.media_type='movie' AND um.status='watchlist'))`, userID, mediaID).Scan(&wasEnabled, &mediaType, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return library.ErrMediaNotFound
	}
	if err != nil {
		return fmt.Errorf("read media notification preference: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE user_media SET notifications_enabled=?,season_alerts_enabled=CASE WHEN ?='tv' THEN 0 ELSE season_alerts_enabled END WHERE user_id=? AND media_id=?`, enabled, mediaType, userID, mediaID)
	if err != nil {
		return fmt.Errorf("set media notification preference: %w", err)
	}
	if mediaType == "tv" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM season_ready_alerts WHERE user_id=? AND ready_at IS NULL AND season_id IN (SELECT id FROM seasons WHERE show_id=?)`, userID, mediaID); err != nil {
			return fmt.Errorf("clear season alerts: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM season_ready_deliveries WHERE user_id=? AND state IN ('pending','failed') AND season_id IN (SELECT id FROM seasons WHERE show_id=?)`, userID, mediaID); err != nil {
			return fmt.Errorf("clear queued season alerts: %w", err)
		}
	}
	if mediaType == "tv" && status == "watching" && enabled && !wasEnabled {
		if _, err := tx.ExecContext(ctx, `UPDATE user_media SET notifications_since=? WHERE user_id=? AND media_id=?`, time.Now().UTC().Format(time.RFC3339Nano), userID, mediaID); err != nil {
			return fmt.Errorf("set show notification baseline: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) SetShowNotificationMode(ctx context.Context, userID, mediaID, mode string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin show notification update: %w", err)
	}
	defer tx.Rollback()
	var status string
	err = tx.QueryRowContext(ctx, `SELECT um.status FROM user_media um JOIN media m ON m.id=um.media_id
		WHERE um.user_id=? AND um.media_id=? AND m.media_type='tv'`, userID, mediaID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return library.ErrMediaNotFound
	}
	if err != nil {
		return fmt.Errorf("read show notification preference: %w", err)
	}
	if status != "watching" {
		return library.ErrMediaNotFound
	}
	episode, season := mode == "episode", mode == "season"
	_, err = tx.ExecContext(ctx, `UPDATE user_media SET notifications_since=CASE
		WHEN notifications_enabled<>? OR season_alerts_enabled<>? THEN ? ELSE notifications_since END,
		notifications_enabled=?,season_alerts_enabled=? WHERE user_id=? AND media_id=?`,
		episode, season, time.Now().UTC().Format(time.RFC3339Nano), episode, season, userID, mediaID)
	if err != nil {
		return fmt.Errorf("set show notification mode: %w", err)
	}
	if episode {
		if _, err := tx.ExecContext(ctx, `DELETE FROM season_ready_alerts WHERE user_id=? AND ready_at IS NULL AND season_id IN (SELECT id FROM seasons WHERE show_id=?)`, userID, mediaID); err != nil {
			return fmt.Errorf("clear season alerts: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM season_ready_deliveries WHERE user_id=? AND state IN ('pending','failed') AND season_id IN (SELECT id FROM seasons WHERE show_id=?)`, userID, mediaID); err != nil {
			return fmt.Errorf("clear queued season alerts: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) GetSeasonAlertsEnabled(ctx context.Context, userID, mediaID string) (bool, error) {
	var enabled bool
	if err := s.DB.QueryRowContext(ctx, `SELECT season_alerts_enabled FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID).Scan(&enabled); err != nil {
		return false, fmt.Errorf("get season notification preference: %w", err)
	}
	return enabled, nil
}

func (s *Store) GetNotificationsEnabled(ctx context.Context, userID, mediaID string) (bool, error) {
	var enabled bool
	if err := s.DB.QueryRowContext(ctx, `SELECT notifications_enabled FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID).Scan(&enabled); err != nil {
		return false, fmt.Errorf("get show notification preference: %w", err)
	}
	return enabled, nil
}

var _ library.Repository = (*Store)(nil)
var _ interface {
	UpsertMedia(context.Context, library.Media) error
	ListItems(context.Context, string) ([]library.Entry, error)
	GetMediaByTMDBID(context.Context, string, domain.MediaType, int64) (library.Entry, error)
} = (*Store)(nil)
