package sqlite

import (
	"context"
	"fmt"
	"time"
)

func (store *Store) CleanupActivityEvents(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if limit < 1 {
		return 0, nil
	}
	result, err := store.DB.ExecContext(ctx, `DELETE FROM activity_events
		WHERE rowid IN (
			SELECT rowid FROM activity_events
			WHERE created_at < ?
			ORDER BY created_at, rowid
			LIMIT ?
		)`, stamp(cutoff), limit)
	if err != nil {
		return 0, fmt.Errorf("delete expired activity events: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted activity events: %w", err)
	}
	return int(removed), nil
}
