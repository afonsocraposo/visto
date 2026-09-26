package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/tracking"
)

// RemoveMediaAndHistory removes one user's relationship with a title and its
// personal history. The shared media and episode catalog is left intact.
func (store *Store) RemoveMediaAndHistory(ctx context.Context, userID, mediaID string) (tracking.RemovedMedia, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return tracking.RemovedMedia{}, fmt.Errorf("begin media removal: %w", err)
	}
	defer tx.Rollback()

	var libraryRating sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT rating FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID).Scan(&libraryRating); err != nil {
		if err == sql.ErrNoRows {
			return tracking.RemovedMedia{}, library.ErrMediaNotFound
		}
		return tracking.RemovedMedia{}, fmt.Errorf("find library entry: %w", err)
	}
	result := tracking.RemovedMedia{MediaID: mediaID}

	playFilter := `user_id=? AND (media_id=? OR episode_id IN (SELECT id FROM episodes WHERE show_id=?))`
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE `+playFilter, userID, mediaID, mediaID).Scan(&result.DeletedPlays); err != nil {
		return tracking.RemovedMedia{}, fmt.Errorf("count media plays: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM episode_ratings WHERE user_id=? AND episode_id IN (SELECT id FROM episodes WHERE show_id=?)`, userID, mediaID).Scan(&result.DeletedRatings); err != nil {
		return tracking.RemovedMedia{}, fmt.Errorf("count episode ratings: %w", err)
	}
	if libraryRating.Valid {
		result.DeletedRatings++
	}

	// Remove direct activity and bulk activity that includes any removed play.
	_, err = tx.ExecContext(ctx, `DELETE FROM activity_events WHERE user_id=? AND (
		media_id=? OR episode_id IN (SELECT id FROM episodes WHERE show_id=?) OR
		play_id IN (SELECT id FROM plays WHERE `+playFilter+`) OR
		(kind='bulk_watch' AND json_valid(detail_json) AND EXISTS (
			SELECT 1 FROM json_each(activity_events.detail_json,'$.play_ids') ids
			JOIN plays p ON CAST(ids.value AS TEXT)=CAST(p.id AS TEXT)
			WHERE p.user_id=? AND (p.media_id=? OR p.episode_id IN (SELECT id FROM episodes WHERE show_id=?))
		))
	)`, userID, mediaID, mediaID, userID, mediaID, mediaID, userID, mediaID, mediaID)
	if err != nil {
		return tracking.RemovedMedia{}, fmt.Errorf("remove media activity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM plays WHERE `+playFilter, userID, mediaID, mediaID); err != nil {
		return tracking.RemovedMedia{}, fmt.Errorf("remove media plays: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM episode_ratings WHERE user_id=? AND episode_id IN (SELECT id FROM episodes WHERE show_id=?)`, userID, mediaID); err != nil {
		return tracking.RemovedMedia{}, fmt.Errorf("remove episode ratings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID); err != nil {
		return tracking.RemovedMedia{}, fmt.Errorf("remove library entry: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return tracking.RemovedMedia{}, fmt.Errorf("commit media removal: %w", err)
	}
	return result, nil
}
