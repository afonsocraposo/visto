package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"github.com/afonsocosta/visto/internal/application/watch"
	"strconv"
	"strings"
	"time"
)

func (store *Store) ListShowEpisodesPage(ctx context.Context, userID, showID string, request pagination.Request) (pagination.Page[watch.ShowEpisode], error) {
	return store.episodePage(ctx, userID, showID, false, request)
}
func (store *Store) ListSeasonEpisodesPage(ctx context.Context, userID, seasonID string, request pagination.Request) (pagination.Page[watch.ShowEpisode], error) {
	return store.episodePage(ctx, userID, seasonID, true, request)
}
func (store *Store) episodePage(ctx context.Context, userID, parentID string, season bool, request pagination.Request) (pagination.Page[watch.ShowEpisode], error) {
	scope := "show-episodes|" + userID + "|" + parentID
	accessQuery := `SELECT EXISTS(SELECT 1 FROM user_media WHERE user_id=? AND media_id=?)`
	column := "e.show_id"
	if season {
		scope = "season-episodes|" + userID + "|" + parentID
		accessQuery = `SELECT EXISTS(SELECT 1 FROM seasons s JOIN user_media um ON um.media_id=s.show_id WHERE s.id=? AND um.user_id=?)`
		column = "e.season_id"
	}
	keys, err := pagination.Decode(request.Cursor, scope, 3)
	if err != nil {
		return pagination.Page[watch.ShowEpisode]{}, err
	}
	var allowed bool
	if season {
		err = store.DB.QueryRowContext(ctx, accessQuery, parentID, userID).Scan(&allowed)
	} else {
		err = store.DB.QueryRowContext(ctx, accessQuery, userID, parentID).Scan(&allowed)
	}
	if err != nil {
		return pagination.Page[watch.ShowEpisode]{}, err
	}
	if !allowed {
		return pagination.Page[watch.ShowEpisode]{}, watch.ErrShowNotFound
	}
	where := ""
	args := []any{userID, parentID}
	if keys != nil {
		seasonNumber, e1 := strconv.Atoi(keys[0])
		episodeNumber, e2 := strconv.Atoi(keys[1])
		if e1 != nil || e2 != nil || seasonNumber < 0 || episodeNumber < 0 {
			return pagination.Page[watch.ShowEpisode]{}, fmt.Errorf("invalid cursor")
		}
		where = " AND (e.season_number>? OR (e.season_number=? AND (e.episode_number>? OR (e.episode_number=? AND e.id>?))))"
		args = append(args, seasonNumber, seasonNumber, episodeNumber, episodeNumber, keys[2])
	}
	args = append(args, request.Limit+1)
	rows, err := store.DB.QueryContext(ctx, `SELECT e.id,e.show_id,e.season_number,e.episode_number,e.air_date,COALESCE(e.name,''),e.overview,e.runtime,e.still_path,EXISTS(SELECT 1 FROM plays p WHERE p.user_id=? AND p.episode_id=e.id) FROM episodes e WHERE `+column+`=?`+where+` ORDER BY e.season_number,e.episode_number,e.id LIMIT ?`, args...)
	if err != nil {
		return pagination.Page[watch.ShowEpisode]{}, err
	}
	defer rows.Close()
	items := []watch.ShowEpisode{}
	for rows.Next() {
		var item watch.ShowEpisode
		var air, overview, still sql.NullString
		var runtime sql.NullInt64
		if err := rows.Scan(&item.Episode.ID, &item.Episode.ShowID, &item.Episode.SeasonNumber, &item.Episode.EpisodeNumber, &air, &item.Name, &overview, &runtime, &still, &item.Watched); err != nil {
			return pagination.Page[watch.ShowEpisode]{}, err
		}
		item.Overview = overview.String
		item.Runtime = int(runtime.Int64)
		item.StillPath = still.String
		if air.Valid && strings.TrimSpace(air.String) != "" {
			date, err := time.Parse(time.DateOnly, air.String)
			if err != nil {
				return pagination.Page[watch.ShowEpisode]{}, err
			}
			item.Episode.AirDate = &date
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Page[watch.ShowEpisode]{}, err
	}
	return pagination.Slice(items, request.Limit, func(item watch.ShowEpisode) string {
		return pagination.Encode(scope, strconv.Itoa(item.Episode.SeasonNumber), strconv.Itoa(item.Episode.EpisodeNumber), item.Episode.ID)
	}), nil
}
