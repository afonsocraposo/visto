package sqlite

import (
	"context"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"strconv"
	"time"
)

func (store *Store) ListPlaysPage(ctx context.Context, userID, mediaID, episodeID string, request pagination.Request) (pagination.Page[tracking.HistoryEntry], error) {
	scope := "plays|" + userID + "|" + mediaID + "|" + episodeID
	keys, err := pagination.Decode(request.Cursor, scope, 2)
	if err != nil {
		return pagination.Page[tracking.HistoryEntry]{}, err
	}
	watchedAt, playID := "", ""
	if keys != nil {
		watchedAt = keys[0]
		if _, err := time.Parse(time.RFC3339Nano, watchedAt); err != nil {
			return pagination.Page[tracking.HistoryEntry]{}, pagination.ErrInvalidCursor
		}
		if _, err := strconv.ParseInt(keys[1], 10, 64); err != nil {
			return pagination.Page[tracking.HistoryEntry]{}, pagination.ErrInvalidCursor
		}
		playID = keys[1]
	}
	entries, err := store.listPlays(ctx, userID, mediaID, request.Limit+1, watchedAt, playID, episodeID)
	if err != nil {
		return pagination.Page[tracking.HistoryEntry]{}, err
	}
	return pagination.Slice(entries, request.Limit, func(item tracking.HistoryEntry) string {
		return pagination.Encode(scope, item.Play.WatchedAt.Format(time.RFC3339Nano), item.Play.ID)
	}), nil
}
