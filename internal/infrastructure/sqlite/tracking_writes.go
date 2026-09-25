package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/domain"
)

func (store *Store) CreatePlay(ctx context.Context, play tracking.Play) (tracking.Play, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return tracking.Play{}, fmt.Errorf("begin play transaction: %w", err)
	}
	defer tx.Rollback()
	var mediaID, episodeID any
	if play.MediaID != nil {
		mediaID = *play.MediaID
	}
	if play.EpisodeID != nil {
		episodeID = *play.EpisodeID
	}
	var existingPlays int
	if play.MediaID != nil {
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND media_id=?`, play.UserID, *play.MediaID).Scan(&existingPlays)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND episode_id=?`, play.UserID, *play.EpisodeID).Scan(&existingPlays)
	}
	if err != nil {
		return tracking.Play{}, fmt.Errorf("check prior plays: %w", err)
	}
	createdAt := time.Now().UTC()
	trackedMediaID := ""
	if play.MediaID != nil {
		trackedMediaID = *play.MediaID
	} else if err := tx.QueryRowContext(ctx, `SELECT show_id FROM episodes WHERE id=?`, *play.EpisodeID).Scan(&trackedMediaID); err != nil {
		return tracking.Play{}, fmt.Errorf("find show for episode play: %w", err)
	}
	if err := ensureWatchingRelationship(ctx, tx, play.UserID, trackedMediaID, createdAt); err != nil {
		return tracking.Play{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO plays(user_id,media_id,episode_id,watched_at,source,created_at) VALUES(?,?,?,?,?,?)`, play.UserID, mediaID, episodeID, play.WatchedAt.Format(time.RFC3339Nano), play.Source, createdAt.Format(time.RFC3339Nano))
	if err != nil {
		return tracking.Play{}, fmt.Errorf("create play: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return tracking.Play{}, fmt.Errorf("get new play ID: %w", err)
	}
	play.ID = strconv.FormatInt(id, 10)
	var visibility string
	if err := tx.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, play.UserID).Scan(&visibility); err != nil {
		return tracking.Play{}, fmt.Errorf("get activity visibility: %w", err)
	}
	if visibility == "instance" {
		kind := "watch"
		if existingPlays > 0 {
			kind = "rewatch"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO activity_events(user_id,kind,play_id,media_id,episode_id,occurred_at,created_at) VALUES(?,?,?,?,?,?,?)`, play.UserID, kind, play.ID, mediaID, episodeID, play.WatchedAt.Format(time.RFC3339Nano), createdAt.Format(time.RFC3339Nano))
		if err != nil {
			return tracking.Play{}, fmt.Errorf("create activity event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return tracking.Play{}, fmt.Errorf("commit play: %w", err)
	}
	return play, nil
}

func (store *Store) CreateBulkPlays(ctx context.Context, plays []tracking.Play) ([]tracking.Play, error) {
	if len(plays) == 0 {
		return nil, fmt.Errorf("bulk play list is empty")
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin bulk play transaction: %w", err)
	}
	defer tx.Rollback()
	plays, err = store.createBulkPlaysInTx(ctx, tx, plays)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit bulk plays: %w", err)
	}
	return plays, nil
}

func (store *Store) createBulkPlaysInTx(ctx context.Context, tx *sql.Tx, plays []tracking.Play) ([]tracking.Play, error) {
	var showID string
	priorWatch := make(map[string]bool, len(plays))
	for index, play := range plays {
		if play.EpisodeID == nil {
			return nil, fmt.Errorf("bulk plays must reference episodes")
		}
		var currentShow string
		if err := tx.QueryRowContext(ctx, `SELECT show_id FROM episodes WHERE id=?`, *play.EpisodeID).Scan(&currentShow); err != nil {
			return nil, fmt.Errorf("find episode for bulk play: %w", err)
		}
		if index == 0 {
			showID = currentShow
		} else if currentShow != showID {
			return nil, fmt.Errorf("bulk episodes must belong to one show")
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND episode_id=?`, play.UserID, *play.EpisodeID).Scan(&count); err != nil {
			return nil, fmt.Errorf("check prior episode watches: %w", err)
		}
		priorWatch[*play.EpisodeID] = count > 0
	}
	createdAtTime := time.Now().UTC()
	createdAt := createdAtTime.Format(time.RFC3339Nano)
	if err := ensureWatchingRelationship(ctx, tx, plays[0].UserID, showID, createdAtTime); err != nil {
		return nil, err
	}
	for index, play := range plays {
		result, err := tx.ExecContext(ctx, `INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES(?,?,?,?,?)`, play.UserID, *play.EpisodeID, play.WatchedAt.Format(time.RFC3339Nano), play.Source, createdAt)
		if err != nil {
			return nil, fmt.Errorf("create bulk play: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("get new bulk play ID: %w", err)
		}
		plays[index].ID = strconv.FormatInt(id, 10)
	}
	var visibility string
	if err := tx.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, plays[0].UserID).Scan(&visibility); err != nil {
		return nil, fmt.Errorf("get activity visibility: %w", err)
	}
	if visibility == "instance" {
		firstWatchIDs := make([]string, 0, len(plays))
		for _, play := range plays {
			if priorWatch[*play.EpisodeID] {
				if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(user_id,kind,play_id,episode_id,occurred_at,created_at) VALUES(?,?,?,?,?,?)`, play.UserID, "rewatch", play.ID, *play.EpisodeID, play.WatchedAt.Format(time.RFC3339Nano), createdAt); err != nil {
					return nil, fmt.Errorf("create episode rewatch activity: %w", err)
				}
			} else {
				firstWatchIDs = append(firstWatchIDs, play.ID)
			}
		}
		if len(firstWatchIDs) > 0 {
			detailJSON, _ := json.Marshal(struct {
				Count   int      `json:"count"`
				PlayIDs []string `json:"play_ids"`
			}{Count: len(firstWatchIDs), PlayIDs: firstWatchIDs})
			detail := string(detailJSON)
			firstPlay := plays[0]
			for _, play := range plays {
				if play.ID == firstWatchIDs[0] {
					firstPlay = play
					break
				}
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(user_id,kind,media_id,detail_json,occurred_at,created_at) VALUES(?,?,?,?,?,?)`, firstPlay.UserID, "bulk_watch", showID, detail, firstPlay.WatchedAt.Format(time.RFC3339Nano), createdAt); err != nil {
				return nil, fmt.Errorf("create bulk activity event: %w", err)
			}
		}
	}
	return plays, nil
}

func (store *Store) MarkEpisodesThrough(ctx context.Context, userID, showID string, season, number int, watchedAt, now time.Time, source string) (int, error) {
	return store.markEpisodesMatching(ctx, userID, showID, watchedAt, now, source, func(episodes []domain.Episode, prior []domain.EpisodePlay, localNow time.Time) ([]domain.Episode, error) {
		for _, episode := range episodes {
			if episode.SeasonNumber == season && episode.EpisodeNumber == number {
				if !episode.IsRegular() || !episode.IsReleasedAt(localNow) {
					break
				}
				return domain.MissingEpisodesThrough(episodes, prior, episode, localNow), nil
			}
		}
		return nil, fmt.Errorf("target must be a released regular episode of this show")
	})
}

func (store *Store) MarkSeasonWatched(ctx context.Context, userID, showID string, season int, watchedAt, now time.Time, source string) (int, error) {
	return store.markEpisodesMatching(ctx, userID, showID, watchedAt, now, source, func(episodes []domain.Episode, prior []domain.EpisodePlay, localNow time.Time) ([]domain.Episode, error) {
		if season <= 0 {
			return nil, fmt.Errorf("season_number must identify a regular season")
		}
		played := make(map[string]bool, len(prior))
		for _, play := range prior {
			played[play.EpisodeID] = true
		}
		missing := []domain.Episode{}
		found := false
		for _, episode := range episodes {
			if episode.SeasonNumber != season {
				continue
			}
			found = true
			if episode.IsReleasedAt(localNow) && !played[episode.ID] {
				missing = append(missing, episode)
			}
		}
		if !found {
			return nil, fmt.Errorf("season is not in this show's catalog")
		}
		return missing, nil
	})
}

func (store *Store) MarkSelectedEpisodes(ctx context.Context, userID, showID string, episodeIDs []string, watchedAt, now time.Time, source string) (int, error) {
	return store.markEpisodesMatching(ctx, userID, showID, watchedAt, now, source, func(episodes []domain.Episode, prior []domain.EpisodePlay, localNow time.Time) ([]domain.Episode, error) {
		requested := make(map[string]bool, len(episodeIDs))
		for _, id := range episodeIDs {
			if id == "" || requested[id] {
				return nil, fmt.Errorf("episode IDs must be non-empty and unique")
			}
			requested[id] = true
		}
		played := make(map[string]bool, len(prior))
		for _, play := range prior {
			played[play.EpisodeID] = true
		}
		missing := []domain.Episode{}
		for _, episode := range episodes {
			if !requested[episode.ID] {
				continue
			}
			delete(requested, episode.ID)
			if !episode.IsRegular() || !episode.IsReleasedAt(localNow) {
				return nil, fmt.Errorf("selected episodes must be released regular episodes")
			}
			if !played[episode.ID] {
				missing = append(missing, episode)
			}
		}
		if len(requested) > 0 {
			return nil, fmt.Errorf("selected episodes must belong to this show")
		}
		return missing, nil
	})
}

func (store *Store) markEpisodesMatching(ctx context.Context, userID, showID string, watchedAt, now time.Time, source string, selectMissing func([]domain.Episode, []domain.EpisodePlay, time.Time) ([]domain.Episode, error)) (int, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin mark-through transaction: %w", err)
	}
	defer tx.Rollback()
	// Take the write lock before selecting watched state, so concurrent repeats
	// cannot both select and insert the same missing episodes.
	result, err := tx.ExecContext(ctx, `UPDATE user_media SET updated_at=updated_at WHERE user_id=? AND media_id=?`, userID, showID)
	if err != nil {
		return 0, fmt.Errorf("check tracked show: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("check tracked show: %w", err)
	}
	if changed == 0 {
		return 0, tracking.ErrShowNotFound
	}
	var zoneName string
	if err := tx.QueryRowContext(ctx, `SELECT timezone FROM user_settings WHERE user_id=?`, userID).Scan(&zoneName); err != nil {
		return 0, fmt.Errorf("get user timezone: %w", err)
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return 0, fmt.Errorf("invalid user timezone: %w", err)
	}
	localNow := now.In(zone)
	rows, err := tx.QueryContext(ctx, `SELECT e.id,e.season_number,e.episode_number,e.air_date,
		EXISTS(SELECT 1 FROM plays p WHERE p.user_id=? AND p.episode_id=e.id)
		FROM episodes e WHERE e.show_id=? ORDER BY e.season_number,e.episode_number`, userID, showID)
	if err != nil {
		return 0, fmt.Errorf("list show episodes: %w", err)
	}
	episodes := []domain.Episode{}
	priorPlays := []domain.EpisodePlay{}
	for rows.Next() {
		var episode domain.Episode
		var airDate sql.NullString
		var watched bool
		if err := rows.Scan(&episode.ID, &episode.SeasonNumber, &episode.EpisodeNumber, &airDate, &watched); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan show episode: %w", err)
		}
		episode.ShowID = showID
		if airDate.Valid && airDate.String != "" {
			parsed, err := time.Parse(time.DateOnly, airDate.String)
			if err != nil {
				rows.Close()
				return 0, fmt.Errorf("parse episode air date: %w", err)
			}
			episode.AirDate = &parsed
		}
		episodes = append(episodes, episode)
		if watched {
			priorPlays = append(priorPlays, domain.EpisodePlay{EpisodeID: episode.ID})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate show episodes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close show episodes: %w", err)
	}
	missing, err := selectMissing(episodes, priorPlays, localNow)
	if err != nil {
		return 0, err
	}
	if len(missing) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE user_media SET status=CASE WHEN status='watchlist' THEN 'watching' ELSE status END, updated_at=? WHERE user_id=? AND media_id=?`, now.Format(time.RFC3339Nano), userID, showID); err != nil {
			return 0, fmt.Errorf("update tracked show: %w", err)
		}
		plays := make([]tracking.Play, 0, len(missing))
		for _, episode := range missing {
			id := episode.ID
			plays = append(plays, tracking.Play{UserID: userID, EpisodeID: &id, WatchedAt: watchedAt, Source: source})
		}
		if _, err := store.createBulkPlaysInTx(ctx, tx, plays); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit mark-through transaction: %w", err)
	}
	return len(missing), nil
}

func ensureWatchingRelationship(ctx context.Context, tx *sql.Tx, userID, mediaID string, createdAt time.Time) error {
	timestamp := createdAt.Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, `INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at,notifications_since)
		VALUES(?,?,?,'watching',?,?,?) ON CONFLICT(user_id,media_id) DO UPDATE SET updated_at=excluded.updated_at,
		status=CASE WHEN excluded.media_id LIKE 'movie:%' AND user_media.status='watchlist' THEN 'watching' ELSE user_media.status END`, userID+":"+mediaID, userID, mediaID, timestamp, timestamp, timestamp)
	if err != nil {
		return fmt.Errorf("ensure library relationship for tracked media: %w", err)
	}
	return nil
}

func (store *Store) UpdatePlay(ctx context.Context, userID, playID string, watchedAt time.Time) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin play correction: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE plays SET watched_at=? WHERE id=? AND user_id=?`, watchedAt.Format(time.RFC3339Nano), playID, userID)
	if err != nil {
		return fmt.Errorf("update play: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect updated play: %w", err)
	}
	if changed == 0 {
		return tracking.ErrPlayNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM activity_events WHERE play_id=? OR (kind='bulk_watch' AND EXISTS(SELECT 1 FROM json_each(activity_events.detail_json,'$.play_ids') WHERE value=?))`, playID, playID); err != nil {
		return fmt.Errorf("remove corrected play activity: %w", err)
	}
	return tx.Commit()
}

func (store *Store) DeletePlay(ctx context.Context, userID, playID string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin play deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM plays WHERE id=? AND user_id=?`, playID, userID)
	if err != nil {
		return fmt.Errorf("delete play: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect deleted play: %w", err)
	}
	if changed == 0 {
		return tracking.ErrPlayNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM activity_events WHERE play_id=? OR (kind='bulk_watch' AND EXISTS(SELECT 1 FROM json_each(activity_events.detail_json,'$.play_ids') WHERE value=?))`, playID, playID); err != nil {
		return fmt.Errorf("remove deleted play activity: %w", err)
	}
	return tx.Commit()
}

func (store *Store) DeleteEpisodePlays(ctx context.Context, userID string, episodeIDs []string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin episode watch removal: %w", err)
	}
	defer tx.Rollback()
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(episodeIDs)), ",")
	arguments := make([]any, 0, len(episodeIDs)+1)
	arguments = append(arguments, userID)
	for _, episodeID := range episodeIDs {
		arguments = append(arguments, episodeID)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM plays WHERE user_id=? AND episode_id IN (`+placeholders+`)`, arguments...)
	if err != nil {
		return fmt.Errorf("list episode watches to remove: %w", err)
	}
	playIDs := []string{}
	for rows.Next() {
		var playID string
		if err := rows.Scan(&playID); err != nil {
			rows.Close()
			return fmt.Errorf("scan episode watch to remove: %w", err)
		}
		playIDs = append(playIDs, playID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate episode watches to remove: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close episode watches: %w", err)
	}
	for _, playID := range playIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM activity_events WHERE play_id=? OR (kind='bulk_watch' AND EXISTS(SELECT 1 FROM json_each(activity_events.detail_json,'$.play_ids') WHERE value=?))`, playID, playID); err != nil {
			return fmt.Errorf("remove activity for episode watch: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM plays WHERE user_id=? AND episode_id IN (`+placeholders+`)`, arguments...); err != nil {
		return fmt.Errorf("remove episode watches: %w", err)
	}
	return tx.Commit()
}

func (store *Store) DeleteMediaPlays(ctx context.Context, userID, mediaID string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin media watch removal: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM plays WHERE user_id=? AND media_id=?`, userID, mediaID)
	if err != nil {
		return fmt.Errorf("list media watches to remove: %w", err)
	}
	playIDs := []string{}
	for rows.Next() {
		var playID string
		if err := rows.Scan(&playID); err != nil {
			rows.Close()
			return fmt.Errorf("scan media watch to remove: %w", err)
		}
		playIDs = append(playIDs, playID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate media watches to remove: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close media watches: %w", err)
	}
	for _, playID := range playIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM activity_events WHERE play_id=? OR (kind='bulk_watch' AND EXISTS(SELECT 1 FROM json_each(activity_events.detail_json,'$.play_ids') WHERE value=?))`, playID, playID); err != nil {
			return fmt.Errorf("remove activity for media watch: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM plays WHERE user_id=? AND media_id=?`, userID, mediaID); err != nil {
		return fmt.Errorf("remove media watches: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID); err != nil {
		return fmt.Errorf("remove movie from library: %w", err)
	}
	return tx.Commit()
}
