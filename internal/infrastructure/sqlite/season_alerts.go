package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/seasonalerts"
)

// seasonReady checks the imported regular episode catalog against its listed count.
const seasonReady = `s.season_number>0 AND s.episode_count>0
	AND (SELECT COUNT(*) FROM episodes e WHERE e.season_id=s.id AND e.season_number>0)>=s.episode_count
	AND NOT EXISTS (SELECT 1 FROM episodes e WHERE e.season_id=s.id AND e.season_number>0
		AND (e.air_date IS NULL OR date(e.air_date) IS NULL OR date(e.air_date)>date(?)))`

func (s *Store) SeasonAlertState(ctx context.Context, userID, seasonID string, now time.Time) (seasonalerts.State, error) {
	var state seasonalerts.State
	err := s.DB.QueryRowContext(ctx, `SELECT ((`+seasonReady+`) OR
		EXISTS(SELECT 1 FROM season_ready_alerts a WHERE a.user_id=? AND a.season_id=s.id AND a.ready_at IS NOT NULL)),
		EXISTS(SELECT 1 FROM season_ready_alerts a WHERE a.user_id=? AND a.season_id=s.id AND a.ready_at IS NULL)
		FROM seasons s JOIN user_media um ON um.media_id=s.show_id AND um.user_id=? WHERE s.id=?`,
		now.Format("2006-01-02"), userID, userID, userID, seasonID).Scan(&state.Ready, &state.Subscribed)
	if errors.Is(err, sql.ErrNoRows) {
		return state, seasonalerts.ErrNotFound
	}
	return state, err
}

func (s *Store) SubscribeSeasonAlert(ctx context.Context, userID, seasonID string, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	var ready bool
	err = tx.QueryRowContext(ctx, `SELECT um.status,(`+seasonReady+`) FROM seasons s
		JOIN user_media um ON um.media_id=s.show_id AND um.user_id=? WHERE s.id=? AND s.season_number>0`,
		now.Format("2006-01-02"), userID, seasonID).Scan(&status, &ready)
	if errors.Is(err, sql.ErrNoRows) {
		return seasonalerts.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "watching" {
		return seasonalerts.ErrNotWatching
	}
	if ready {
		return seasonalerts.ErrAlreadyReady
	}
	var existingReady sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT ready_at FROM season_ready_alerts WHERE user_id=? AND season_id=?`, userID, seasonID).Scan(&existingReady)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if existingReady.Valid {
		return seasonalerts.ErrAlreadyReady
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO season_ready_alerts(user_id,season_id,created_at) VALUES(?,?,?)
		ON CONFLICT(user_id,season_id) DO NOTHING`, userID, seasonID, now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CancelSeasonAlert(ctx context.Context, userID, seasonID string) error {
	_, err := s.SeasonAlertState(ctx, userID, seasonID, time.Now().UTC())
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM season_ready_alerts WHERE user_id=? AND season_id=? AND ready_at IS NULL`, userID, seasonID)
	return err
}

func (s *Store) PendingSeasonNumbers(ctx context.Context, showID string) ([]int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT se.season_number FROM season_ready_alerts a
		JOIN seasons se ON se.id=a.season_id WHERE se.show_id=? AND a.ready_at IS NULL ORDER BY se.season_number`, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var numbers []int
	for rows.Next() {
		var number int
		if err := rows.Scan(&number); err != nil {
			return nil, err
		}
		numbers = append(numbers, number)
	}
	return numbers, rows.Err()
}

func (s *Store) MarkReadySeasonAlerts(ctx context.Context, now time.Time, pushover, web bool, limit int) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO season_ready_alerts(user_id,season_id,created_at)
		SELECT um.user_id,s.id,? FROM user_media um JOIN seasons s ON s.show_id=um.media_id
		WHERE um.status='watching' AND um.season_alerts_enabled=1 AND s.season_number>0
		AND EXISTS(SELECT 1 FROM episodes e WHERE e.season_id=s.id
			AND (e.air_date IS NULL OR date(e.air_date)>=date(COALESCE(um.notifications_since,um.added_at))))
		ON CONFLICT(user_id,season_id) DO NOTHING`, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("register show season alerts: %w", err)
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT a.user_id,a.season_id FROM season_ready_alerts a
		JOIN seasons s ON s.id=a.season_id JOIN user_media um ON um.user_id=a.user_id AND um.media_id=s.show_id
		WHERE a.ready_at IS NULL AND um.status='watching' AND `+seasonReady+`
		ORDER BY a.created_at LIMIT ?`, now.Format("2006-01-02"), limit)
	if err != nil {
		return err
	}
	type pair struct{ userID, seasonID string }
	var pending []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.userID, &p.seasonID); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, p := range pending {
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		stamp := now.UTC().Format(time.RFC3339Nano)
		result, err := tx.ExecContext(ctx, `UPDATE season_ready_alerts SET ready_at=? WHERE user_id=? AND season_id=? AND ready_at IS NULL
			AND EXISTS(SELECT 1 FROM seasons s JOIN user_media um ON um.media_id=s.show_id AND um.user_id=?
				WHERE s.id=? AND um.status='watching' AND `+seasonReady+`)`, stamp, p.userID, p.seasonID, p.userID, p.seasonID, now.Format("2006-01-02"))
		if err != nil {
			tx.Rollback()
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			tx.Rollback()
			return err
		}
		if changed == 0 {
			tx.Rollback()
			continue
		}
		if pushover {
			_, err = tx.ExecContext(ctx, `INSERT INTO season_ready_deliveries(user_id,season_id,channel,subscription_id,state)
				SELECT ?,?,'pushover','','pending' FROM user_settings us WHERE us.user_id=?
				AND us.pushover_notifications_enabled=1 AND us.pushover_app_token_encrypted IS NOT NULL
				AND us.pushover_user_key_encrypted IS NOT NULL`, p.userID, p.seasonID, p.userID)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
		if web {
			_, err = tx.ExecContext(ctx, `INSERT INTO season_ready_deliveries(user_id,season_id,channel,subscription_id,state)
				SELECT ?,?,'web_push',ps.id,'pending' FROM web_push_subscriptions ps WHERE ps.user_id=?`, p.userID, p.seasonID, p.userID)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DueSeasonDeliveries(ctx context.Context, now time.Time, limit int) ([]seasonalerts.Delivery, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT d.user_id,d.season_id,d.channel,d.subscription_id,
		COALESCE(us.pushover_app_token_encrypted,''),COALESCE(us.pushover_user_key_encrypted,''),
		COALESCE(ps.encrypted_subscription,''),m.title,se.season_number,m.tmdb_id,d.attempt_count
		FROM season_ready_deliveries d JOIN season_ready_alerts a ON a.user_id=d.user_id AND a.season_id=d.season_id
		JOIN seasons se ON se.id=d.season_id JOIN media m ON m.id=se.show_id
		JOIN user_media um ON um.user_id=d.user_id AND um.media_id=m.id AND um.status='watching'
		LEFT JOIN user_settings us ON us.user_id=d.user_id AND d.channel='pushover'
		LEFT JOIN web_push_subscriptions ps ON ps.id=d.subscription_id AND ps.user_id=d.user_id AND d.channel='web_push'
		WHERE (d.state='pending' OR (d.state='failed' AND d.attempt_count<? AND d.next_attempt_at<=?))
		AND ((d.channel='pushover' AND us.pushover_notifications_enabled=1 AND us.pushover_app_token_encrypted IS NOT NULL AND us.pushover_user_key_encrypted IS NOT NULL)
		OR (d.channel='web_push' AND ps.id IS NOT NULL))
		ORDER BY a.ready_at,d.channel,d.subscription_id LIMIT ?`, 3, now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []seasonalerts.Delivery
	for rows.Next() {
		var d seasonalerts.Delivery
		if err := rows.Scan(&d.UserID, &d.SeasonID, &d.Channel, &d.SubscriptionID, &d.EncryptedAppToken, &d.EncryptedUserKey, &d.EncryptedSubscription, &d.ShowTitle, &d.SeasonNumber, &d.TMDBID, &d.AttemptCount); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) ClaimSeasonDelivery(ctx context.Context, d seasonalerts.Delivery, now time.Time) (bool, error) {
	result, err := s.DB.ExecContext(ctx, `UPDATE season_ready_deliveries SET state='sending',attempt_count=attempt_count+1,attempted_at=?,next_attempt_at=NULL
		WHERE user_id=? AND season_id=? AND channel=? AND subscription_id=?
		AND (state='pending' OR (state='failed' AND attempt_count<3 AND next_attempt_at<=?))`,
		now.UTC().Format(time.RFC3339Nano), d.UserID, d.SeasonID, d.Channel, d.SubscriptionID, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (s *Store) FinishSeasonDelivery(ctx context.Context, d seasonalerts.Delivery, now time.Time, failed bool, next *time.Time) error {
	state := "sent"
	var sent, retry any = now.UTC().Format(time.RFC3339Nano), nil
	if failed {
		state, sent = "failed", nil
		if next != nil {
			retry = next.UTC().Format(time.RFC3339Nano)
		}
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE season_ready_deliveries SET state=?,attempted_at=?,sent_at=?,next_attempt_at=?
		WHERE user_id=? AND season_id=? AND channel=? AND subscription_id=? AND state='sending'`,
		state, now.UTC().Format(time.RFC3339Nano), sent, retry, d.UserID, d.SeasonID, d.Channel, d.SubscriptionID)
	return err
}

func (s *Store) RemoveSeasonWebDelivery(ctx context.Context, d seasonalerts.Delivery) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM web_push_subscriptions WHERE user_id=? AND id=?`, d.UserID, d.SubscriptionID)
	if err != nil {
		return fmt.Errorf("remove expired push subscription: %w", err)
	}
	return nil
}

var _ seasonalerts.Repository = (*Store)(nil)
