package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/notifications"
)

func (s *Store) PrepareMovieReleaseDeliveries(ctx context.Context, now time.Time, pushover, web bool, limit int) error {
	day := now.UTC().Format("2006-01-02")
	if pushover {
		_, err := s.DB.ExecContext(ctx, `INSERT INTO movie_release_deliveries(user_id,media_id,channel,subscription_id,state)
			SELECT um.user_id,m.id,'pushover','','pending' FROM user_media um JOIN media m ON m.id=um.media_id
			JOIN user_settings us ON us.user_id=um.user_id
			WHERE m.media_type='movie' AND um.status='watchlist' AND um.notifications_enabled=1
			AND us.pushover_notifications_enabled=1 AND us.pushover_app_token_encrypted IS NOT NULL AND us.pushover_user_key_encrypted IS NOT NULL
			AND m.release_date IS NOT NULL AND date(m.release_date)<=date(?) AND date(m.release_date)>=date(um.added_at)
			AND NOT EXISTS (SELECT 1 FROM movie_release_deliveries d WHERE d.user_id=um.user_id AND d.media_id=m.id
				AND d.channel='pushover' AND (d.state IN ('sending','sent') OR d.attempt_count>=3 OR d.state='failed'))
			ORDER BY date(m.release_date),m.id LIMIT ? ON CONFLICT DO NOTHING`, day, limit)
		if err != nil {
			return fmt.Errorf("prepare Pushover movie release alerts: %w", err)
		}
	}
	if web {
		_, err := s.DB.ExecContext(ctx, `INSERT INTO movie_release_deliveries(user_id,media_id,channel,subscription_id,state)
			SELECT um.user_id,m.id,'web_push',ps.id,'pending' FROM user_media um JOIN media m ON m.id=um.media_id
			JOIN web_push_subscriptions ps ON ps.user_id=um.user_id
			WHERE m.media_type='movie' AND um.status='watchlist' AND um.notifications_enabled=1
			AND m.release_date IS NOT NULL AND date(m.release_date)<=date(?) AND date(m.release_date)>=date(um.added_at)
			AND NOT EXISTS (SELECT 1 FROM movie_release_deliveries d WHERE d.user_id=um.user_id AND d.media_id=m.id
				AND d.channel='web_push' AND d.subscription_id=ps.id AND (d.state IN ('sending','sent') OR d.attempt_count>=3 OR d.state='failed'))
			ORDER BY date(m.release_date),m.id,ps.id LIMIT ? ON CONFLICT DO NOTHING`, day, limit)
		if err != nil {
			return fmt.Errorf("prepare Web Push movie release alerts: %w", err)
		}
	}
	return nil
}

func (s *Store) MovieReleaseDeliveries(ctx context.Context, now time.Time, limit int) ([]notifications.MovieReleaseDelivery, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT d.user_id,d.media_id,d.channel,d.subscription_id,
		COALESCE(us.pushover_app_token_encrypted,''),COALESCE(us.pushover_user_key_encrypted,''),
		COALESCE(ps.encrypted_subscription,''),m.title,m.tmdb_id,d.attempt_count
		FROM movie_release_deliveries d JOIN media m ON m.id=d.media_id
		JOIN user_media um ON um.user_id=d.user_id AND um.media_id=d.media_id
		LEFT JOIN user_settings us ON us.user_id=d.user_id AND d.channel='pushover'
		LEFT JOIN web_push_subscriptions ps ON ps.user_id=d.user_id AND ps.id=d.subscription_id AND d.channel='web_push'
		WHERE m.media_type='movie' AND um.status='watchlist' AND um.notifications_enabled=1
		AND m.release_date IS NOT NULL AND date(m.release_date)<=date(?) AND date(m.release_date)>=date(um.added_at)
		AND (d.state='pending' OR (d.state='failed' AND d.attempt_count<3 AND d.next_attempt_at<=?))
		AND ((d.channel='pushover' AND us.pushover_notifications_enabled=1 AND us.pushover_app_token_encrypted IS NOT NULL AND us.pushover_user_key_encrypted IS NOT NULL)
		OR (d.channel='web_push' AND ps.id IS NOT NULL))
		ORDER BY date(m.release_date),m.id,d.channel,d.subscription_id LIMIT ?`, now.UTC().Format("2006-01-02"), now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []notifications.MovieReleaseDelivery
	for rows.Next() {
		var d notifications.MovieReleaseDelivery
		if err := rows.Scan(&d.UserID, &d.MediaID, &d.Channel, &d.SubscriptionID, &d.EncryptedAppToken, &d.EncryptedUserKey, &d.EncryptedSubscription, &d.Title, &d.TMDBID, &d.AttemptCount); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) ClaimMovieReleaseDelivery(ctx context.Context, d notifications.MovieReleaseDelivery, now time.Time) (bool, error) {
	result, err := s.DB.ExecContext(ctx, `UPDATE movie_release_deliveries SET state='sending',attempt_count=attempt_count+1,attempted_at=?,next_attempt_at=NULL
		WHERE user_id=? AND media_id=? AND channel=? AND subscription_id=?
		AND (state='pending' OR (state='failed' AND attempt_count<3 AND next_attempt_at<=?))`,
		now.UTC().Format(time.RFC3339Nano), d.UserID, d.MediaID, d.Channel, d.SubscriptionID, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (s *Store) FinishMovieReleaseDelivery(ctx context.Context, d notifications.MovieReleaseDelivery, now time.Time, failed bool, next *time.Time) error {
	state := "sent"
	var sent, retry any = now.UTC().Format(time.RFC3339Nano), nil
	if failed {
		state, sent = "failed", nil
		if next != nil {
			retry = next.UTC().Format(time.RFC3339Nano)
		}
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE movie_release_deliveries SET state=?,attempted_at=?,sent_at=?,next_attempt_at=?
		WHERE user_id=? AND media_id=? AND channel=? AND subscription_id=? AND state='sending'`,
		state, now.UTC().Format(time.RFC3339Nano), sent, retry, d.UserID, d.MediaID, d.Channel, d.SubscriptionID)
	return err
}

func (s *Store) RemoveMovieReleaseWebDelivery(ctx context.Context, d notifications.MovieReleaseDelivery) error {
	return s.DeletePushSubscription(ctx, d.UserID, d.SubscriptionID)
}

var _ notifications.MovieReleaseRepository = (*Store)(nil)
