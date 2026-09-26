package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/feed"
)

func (store *Store) List(ctx context.Context, cursor string, limit int) (feed.Page, error) {
	where, arguments, err := feedCursor(cursor)
	if err != nil {
		return feed.Page{}, err
	}
	arguments = append(arguments, limit+1)
	rows, err := store.DB.QueryContext(ctx, `SELECT ae.id,u.display_name,ae.kind,COALESCE(media.title,show.title),COALESCE(media.media_type,show.media_type,''),COALESCE(media.tmdb_id,show.tmdb_id,0),COALESCE(episode.still_path,media.poster_path,show.poster_path,''),ae.rating,COALESCE(json_extract(ae.detail_json,'$.count'),0),episode.season_number,episode.episode_number,episode.name,ae.occurred_at
		FROM activity_events ae
		JOIN users u ON u.id=ae.user_id
		JOIN user_settings settings ON settings.user_id=u.id AND settings.activity_visibility='instance'
		LEFT JOIN media ON media.id=ae.media_id
		LEFT JOIN episodes episode ON episode.id=ae.episode_id
		LEFT JOIN media show ON show.id=episode.show_id
		`+where+` ORDER BY ae.occurred_at DESC,ae.id DESC LIMIT ?`, arguments...)
	if err != nil {
		return feed.Page{}, fmt.Errorf("list feed: %w", err)
	}
	defer rows.Close()
	items := []feed.Item{}
	for rows.Next() {
		var item feed.Item
		var rating, season, episode sql.NullInt64
		var mediaType, artworkPath, episodeName sql.NullString
		var occurredAt string
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Kind, &item.Title, &mediaType, &item.TMDBID, &artworkPath, &rating, &item.Count, &season, &episode, &episodeName, &occurredAt); err != nil {
			return feed.Page{}, fmt.Errorf("scan feed item: %w", err)
		}
		if mediaType.Valid {
			item.MediaType = mediaType.String
		}
		if artworkPath.Valid {
			item.ArtworkPath = artworkPath.String
		}
		if episodeName.Valid {
			item.EpisodeName = episodeName.String
		}
		if season.Valid {
			value := int(season.Int64)
			item.SeasonNumber = &value
		}
		if rating.Valid {
			value := int(rating.Int64)
			item.Rating = &value
		}
		if episode.Valid {
			value := int(episode.Int64)
			item.EpisodeNumber = &value
		}
		item.OccurredAt, err = time.Parse(time.RFC3339Nano, occurredAt)
		if err != nil {
			return feed.Page{}, fmt.Errorf("parse feed time: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return feed.Page{}, fmt.Errorf("iterate feed: %w", err)
	}
	page := feed.Page{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		value := page.Items[len(page.Items)-1].OccurredAt.Format(time.RFC3339Nano) + "|" + page.Items[len(page.Items)-1].ID
		page.NextCursor = &value
	}
	return page, nil
}

func feedCursor(cursor string) (string, []any, error) {
	if cursor == "" {
		return "", nil, nil
	}
	parts := strings.SplitN(cursor, "|", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid feed cursor")
	}
	if _, err := time.Parse(time.RFC3339Nano, parts[0]); err != nil {
		return "", nil, fmt.Errorf("invalid feed cursor")
	}
	return "WHERE (ae.occurred_at < ? OR (ae.occurred_at = ? AND ae.id < ?))", []any{parts[0], parts[0], parts[1]}, nil
}

var _ feed.Repository = (*Store)(nil)
