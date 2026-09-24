package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/notifications"
)

func (store *Store) NotificationCandidates(ctx context.Context, today string, limit int) ([]notifications.Candidate, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT um.user_id,e.id,us.pushover_user_key_encrypted,m.title,
		COALESCE(e.name,''),e.season_number,e.episode_number
		FROM user_media um
		JOIN media m ON m.id=um.media_id AND m.media_type='tv'
		JOIN user_settings us ON us.user_id=um.user_id
		JOIN episodes e ON e.show_id=m.id
		WHERE um.status='watching' AND um.notifications_enabled=1
			AND us.pushover_notifications_enabled=1 AND us.pushover_user_key_encrypted IS NOT NULL
			AND e.season_number>0 AND e.air_date IS NOT NULL
			AND date(e.air_date)<=date(?) AND date(e.air_date)>=date(COALESCE(um.notifications_since,um.added_at))
			AND NOT EXISTS (SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.episode_id=e.id)
			AND NOT EXISTS (SELECT 1 FROM notification_deliveries d WHERE d.user_id=um.user_id AND d.episode_id=e.id)
		ORDER BY e.air_date,e.show_id,e.season_number,e.episode_number LIMIT ?`, today, limit)
	if err != nil {
		return nil, fmt.Errorf("query notification candidates: %w", err)
	}
	defer rows.Close()
	var candidates []notifications.Candidate
	for rows.Next() {
		var candidate notifications.Candidate
		if err := rows.Scan(&candidate.UserID, &candidate.EpisodeID, &candidate.EncryptedUserKey, &candidate.ShowTitle, &candidate.EpisodeName, &candidate.SeasonNumber, &candidate.EpisodeNumber); err != nil {
			return nil, fmt.Errorf("read notification candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification candidates: %w", err)
	}
	return candidates, nil
}

func (store *Store) ClaimNotification(ctx context.Context, userID, episodeID string, attemptedAt time.Time) (bool, error) {
	result, err := store.DB.ExecContext(ctx, `INSERT INTO notification_deliveries(user_id,episode_id,state,attempted_at)
		VALUES(?,?,'sending',?) ON CONFLICT(user_id,episode_id) DO NOTHING`, userID, episodeID, attemptedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, fmt.Errorf("claim episode notification: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("inspect episode notification claim: %w", err)
	}
	return changed == 1, nil
}

func (store *Store) CompleteNotification(ctx context.Context, userID, episodeID string, sentAt time.Time) error {
	return store.setNotificationState(ctx, userID, episodeID, "sent", sentAt, sentAt)
}

func (store *Store) FailNotification(ctx context.Context, userID, episodeID string, attemptedAt time.Time) error {
	return store.setNotificationState(ctx, userID, episodeID, "failed", attemptedAt, time.Time{})
}

func (store *Store) setNotificationState(ctx context.Context, userID, episodeID, state string, attemptedAt, sentAt time.Time) error {
	var sentAtValue sql.NullString
	if !sentAt.IsZero() {
		sentAtValue = sql.NullString{String: sentAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	result, err := store.DB.ExecContext(ctx, `UPDATE notification_deliveries SET state=?,attempted_at=?,sent_at=?
		WHERE user_id=? AND episode_id=? AND state='sending'`, state, attemptedAt.UTC().Format(time.RFC3339Nano), sentAtValue, userID, episodeID)
	if err != nil {
		return fmt.Errorf("update episode notification state: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect episode notification state: %w", err)
	}
	if changed != 1 {
		return fmt.Errorf("notification claim not found")
	}
	return nil
}

var _ notifications.Repository = (*Store)(nil)
