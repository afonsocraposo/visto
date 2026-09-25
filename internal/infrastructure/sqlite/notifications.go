package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/notifications"
)

func (store *Store) NotificationCandidates(ctx context.Context, now time.Time, limit int) ([]notifications.Candidate, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT um.user_id,e.id,us.pushover_app_token_encrypted,us.pushover_user_key_encrypted,m.title,
		COALESCE(e.name,''),e.season_number,e.episode_number
		FROM user_media um
		JOIN media m ON m.id=um.media_id AND m.media_type='tv'
		JOIN user_settings us ON us.user_id=um.user_id
		JOIN episodes e ON e.show_id=m.id
		WHERE um.status='watching' AND um.notifications_enabled=1
			AND us.pushover_notifications_enabled=1 AND us.pushover_app_token_encrypted IS NOT NULL AND us.pushover_user_key_encrypted IS NOT NULL
			AND e.season_number>0 AND e.air_date IS NOT NULL
			AND date(e.air_date)<=date(?) AND date(e.air_date)>=date(COALESCE(um.notifications_since,um.added_at))
			AND NOT EXISTS (SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.episode_id=e.id)
			AND NOT EXISTS (SELECT 1 FROM notification_deliveries d WHERE d.user_id=um.user_id AND d.episode_id=e.id
				AND (d.state IN ('sending','sent') OR d.attempt_count>=? OR d.next_attempt_at IS NULL OR d.next_attempt_at>?))
		ORDER BY e.air_date,e.show_id,e.season_number,e.episode_number LIMIT ?`, now.UTC().Format("2006-01-02"), notifications.MaxDeliveryAttempts, now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, fmt.Errorf("query notification candidates: %w", err)
	}
	defer rows.Close()
	var candidates []notifications.Candidate
	for rows.Next() {
		var candidate notifications.Candidate
		if err := rows.Scan(&candidate.UserID, &candidate.EpisodeID, &candidate.EncryptedAppToken, &candidate.EncryptedUserKey, &candidate.ShowTitle, &candidate.EpisodeName, &candidate.SeasonNumber, &candidate.EpisodeNumber); err != nil {
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
	now := attemptedAt.UTC().Format(time.RFC3339Nano)
	result, err := store.DB.ExecContext(ctx, `INSERT INTO notification_deliveries(user_id,episode_id,state,attempted_at,attempt_count)
		VALUES(?,?,'sending',?,1) ON CONFLICT(user_id,episode_id) DO UPDATE SET
		state='sending',attempted_at=excluded.attempted_at,attempt_count=notification_deliveries.attempt_count+1,
		next_attempt_at=NULL,sent_at=NULL
		WHERE notification_deliveries.state='failed' AND notification_deliveries.attempt_count<?
			AND notification_deliveries.next_attempt_at<=excluded.attempted_at`, userID, episodeID, now, notifications.MaxDeliveryAttempts)
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
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin failed notification update: %w", err)
	}
	defer tx.Rollback()
	var attempt int
	if err := tx.QueryRowContext(ctx, `SELECT attempt_count FROM notification_deliveries WHERE user_id=? AND episode_id=? AND state='sending'`, userID, episodeID).Scan(&attempt); err != nil {
		return fmt.Errorf("read failed notification attempt: %w", err)
	}
	var nextAttempt any
	if delay := notifications.RetryDelay(attempt); delay > 0 {
		nextAttempt = attemptedAt.UTC().Add(delay).Format(time.RFC3339Nano)
	}
	result, err := tx.ExecContext(ctx, `UPDATE notification_deliveries SET state='failed',attempted_at=?,next_attempt_at=?,sent_at=NULL
		WHERE user_id=? AND episode_id=? AND state='sending'`, attemptedAt.UTC().Format(time.RFC3339Nano), nextAttempt, userID, episodeID)
	if err != nil {
		return fmt.Errorf("record failed notification: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect failed notification state: %w", err)
	}
	if changed != 1 {
		return fmt.Errorf("notification claim not found")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed notification update: %w", err)
	}
	return nil
}

func (store *Store) setNotificationState(ctx context.Context, userID, episodeID, state string, attemptedAt, sentAt time.Time) error {
	var sentAtValue sql.NullString
	if !sentAt.IsZero() {
		sentAtValue = sql.NullString{String: sentAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	result, err := store.DB.ExecContext(ctx, `UPDATE notification_deliveries SET state=?,attempted_at=?,sent_at=?,next_attempt_at=NULL
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
