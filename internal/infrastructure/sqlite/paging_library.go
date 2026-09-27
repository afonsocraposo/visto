package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/pagination"
)

func (s *Store) ListItemsPage(ctx context.Context, userID string, options library.ListOptions, request pagination.Request) (pagination.Page[library.Entry], error) {
	scope := "library|" + userID + "|" + options.Sort + "|" + options.Status + "|" + options.MediaType
	keys, err := pagination.Decode(request.Cursor, scope, 3)
	if err != nil {
		return pagination.Page[library.Entry]{}, err
	}
	if keys != nil {
		if !strings.HasPrefix(keys[2], "movie:") && !strings.HasPrefix(keys[2], "tv:") {
			return pagination.Page[library.Entry]{}, pagination.ErrInvalidCursor
		}
		if options.Sort == "updated" {
			if _, err := time.Parse(time.RFC3339Nano, keys[0]); err != nil {
				return pagination.Page[library.Entry]{}, pagination.ErrInvalidCursor
			}
		}
		if options.Sort == "released" && keys[0] != "0000-00-00" {
			if _, err := time.Parse(time.DateOnly, keys[0]); err != nil {
				return pagination.Page[library.Entry]{}, pagination.ErrInvalidCursor
			}
		}
	}
	order, predicate := libraryOrder(options.Sort)
	if keys == nil {
		predicate = "1=1"
	}
	var totalCount int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_media um JOIN media m ON m.id=um.media_id
		WHERE um.user_id=? AND (?='' OR m.media_type=?) AND (?='' OR um.status=?)`,
		userID, options.MediaType, options.MediaType, options.Status, options.Status).Scan(&totalCount); err != nil {
		return pagination.Page[library.Entry]{}, fmt.Errorf("count library items: %w", err)
	}
	query := `WITH candidates AS (
 SELECT um.media_id,um.updated_at,m.title,COALESCE(NULLIF(m.release_date,''),'0000-00-00') AS released,m.media_type,um.status
 FROM user_media um JOIN media m ON m.id=um.media_id WHERE um.user_id=?),
 filtered AS (SELECT * FROM candidates WHERE (?='' OR media_type=?) AND (?='' OR status=?))
 SELECT media_id,updated_at,title,released FROM filtered WHERE ` + predicate + ` ORDER BY ` + order + ` LIMIT ?`
	args := []any{userID, options.MediaType, options.MediaType, options.Status, options.Status}
	if keys != nil {
		switch options.Sort {
		case "title":
			args = append(args, keys[0], keys[0], keys[2])
		default:
			args = append(args, keys[0], keys[0], keys[1], keys[1], keys[2])
		}
	}
	args = append(args, request.Limit+1)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return pagination.Page[library.Entry]{}, fmt.Errorf("list library page: %w", err)
	}
	type position struct{ id, updated, title, released string }
	found := []position{}
	for rows.Next() {
		var p position
		if err := rows.Scan(&p.id, &p.updated, &p.title, &p.released); err != nil {
			rows.Close()
			return pagination.Page[library.Entry]{}, err
		}
		found = append(found, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return pagination.Page[library.Entry]{}, err
	}
	more := len(found) > request.Limit
	if more {
		found = found[:request.Limit]
	}
	ids := make([]string, len(found))
	for i, p := range found {
		ids[i] = p.id
	}
	page := pagination.Page[library.Entry]{Items: []library.Entry{}, TotalCount: &totalCount}
	if len(ids) == 0 {
		return page, nil
	}
	entries, err := s.listItemsSorted(ctx, userID, options.Sort, ids)
	if err != nil {
		return page, err
	}
	byID := map[string]library.Entry{}
	for _, entry := range entries {
		byID[entry.Item.MediaID] = entry
	}
	for _, id := range ids {
		page.Items = append(page.Items, byID[id])
	}
	if more {
		last := found[len(found)-1]
		primary := last.updated
		if options.Sort == "title" {
			primary = last.title
		}
		if options.Sort == "released" {
			primary = last.released
		}
		next := pagination.Encode(scope, primary, last.title, last.id)
		page.NextCursor = &next
	}
	return page, nil
}

func libraryOrder(sort string) (string, string) {
	switch sort {
	case "title":
		return "title COLLATE NOCASE,media_id", "(title COLLATE NOCASE > ? COLLATE NOCASE OR (title COLLATE NOCASE = ? COLLATE NOCASE AND media_id>?))"
	case "released":
		return "released DESC,title COLLATE NOCASE,media_id", "(released<? OR (released=? AND (title COLLATE NOCASE > ? COLLATE NOCASE OR (title COLLATE NOCASE = ? COLLATE NOCASE AND media_id>?))))"
	default:
		return "updated_at DESC,title COLLATE NOCASE,media_id", "(updated_at<? OR (updated_at=? AND (title COLLATE NOCASE > ? COLLATE NOCASE OR (title COLLATE NOCASE = ? COLLATE NOCASE AND media_id>?))))"
	}
}
