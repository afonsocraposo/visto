package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/notifications"
)

func PushSubscriptionID(endpoint string) string {
	sum := sha256.Sum256([]byte(endpoint))
	return hex.EncodeToString(sum[:])
}

func (s *Store) SavePushSubscription(ctx context.Context, userID, id, encrypted string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO web_push_subscriptions(id,user_id,encrypted_subscription,created_at) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET user_id=excluded.user_id,encrypted_subscription=excluded.encrypted_subscription,created_at=excluded.created_at`, id, userID, encrypted, now)
	return err
}
func (s *Store) HasPushSubscription(ctx context.Context, userID, id string) (bool, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM web_push_subscriptions WHERE user_id=? AND id=?`, userID, id).Scan(&count)
	return count > 0, err
}
func (s *Store) DeletePushSubscription(ctx context.Context, userID, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM web_push_subscriptions WHERE user_id=? AND id=?`, userID, id)
	return err
}
func (s *Store) WebPushCandidates(ctx context.Context, now time.Time, limit int) ([]notifications.WebCandidate, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT ps.user_id,e.id,ps.id,ps.encrypted_subscription,m.title,COALESCE(e.name,''),e.season_number,e.episode_number,m.tmdb_id
 FROM web_push_subscriptions ps JOIN user_media um ON um.user_id=ps.user_id AND um.status='watching' AND um.notifications_enabled=1
 JOIN media m ON m.id=um.media_id AND m.media_type='tv' JOIN episodes e ON e.show_id=m.id
 WHERE e.active=1 AND e.season_number>0 AND e.air_date IS NOT NULL AND date(e.air_date)<=date(?)
 AND NOT EXISTS (SELECT 1 FROM season_ready_alerts a WHERE a.user_id=ps.user_id AND a.season_id=e.season_id)
 AND date(e.air_date)>=date(ps.created_at) AND date(e.air_date)>=date(COALESCE(um.notifications_since,um.added_at))
 AND NOT EXISTS (SELECT 1 FROM plays p WHERE p.user_id=ps.user_id AND p.episode_id=e.id)
 AND NOT EXISTS (SELECT 1 FROM web_push_deliveries d WHERE d.user_id=ps.user_id AND d.episode_id=e.id AND d.subscription_id=ps.id
 AND (d.state IN ('sending','sent') OR d.attempt_count>=? OR d.next_attempt_at IS NULL OR d.next_attempt_at>?))
 ORDER BY e.air_date,e.show_id,e.season_number,e.episode_number LIMIT ?`, now.UTC().Format("2006-01-02"), notifications.MaxDeliveryAttempts, now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []notifications.WebCandidate
	for rows.Next() {
		var c notifications.WebCandidate
		if err := rows.Scan(&c.UserID, &c.EpisodeID, &c.SubscriptionID, &c.EncryptedSubscription, &c.ShowTitle, &c.EpisodeName, &c.SeasonNumber, &c.EpisodeNumber, &c.TMDBID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) ClaimWebPush(ctx context.Context, userID, episodeID, subscriptionID string, at time.Time) (bool, error) {
	result, err := s.DB.ExecContext(ctx, `INSERT INTO web_push_deliveries(user_id,episode_id,subscription_id,state,attempted_at,attempt_count) VALUES(?,?,?,'sending',?,1)
 ON CONFLICT(user_id,episode_id,subscription_id) DO UPDATE SET state='sending',attempted_at=excluded.attempted_at,attempt_count=web_push_deliveries.attempt_count+1,next_attempt_at=NULL,sent_at=NULL
 WHERE web_push_deliveries.state='failed' AND web_push_deliveries.attempt_count<? AND web_push_deliveries.next_attempt_at<=excluded.attempted_at`, userID, episodeID, subscriptionID, at.UTC().Format(time.RFC3339Nano), notifications.MaxDeliveryAttempts)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (s *Store) CompleteWebPush(ctx context.Context, userID, episodeID, subscriptionID string, at time.Time) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE web_push_deliveries SET state='sent',attempted_at=?,sent_at=?,next_attempt_at=NULL WHERE user_id=? AND episode_id=? AND subscription_id=? AND state='sending'`, at.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano), userID, episodeID, subscriptionID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("web push claim not found")
	}
	return nil
}
func (s *Store) FailWebPush(ctx context.Context, userID, episodeID, subscriptionID string, at time.Time) error {
	var attempt int
	if err := s.DB.QueryRowContext(ctx, `SELECT attempt_count FROM web_push_deliveries WHERE user_id=? AND episode_id=? AND subscription_id=? AND state='sending'`, userID, episodeID, subscriptionID).Scan(&attempt); err != nil {
		return err
	}
	var next any
	if d := notifications.RetryDelay(attempt); d > 0 {
		next = at.UTC().Add(d).Format(time.RFC3339Nano)
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE web_push_deliveries SET state='failed',attempted_at=?,next_attempt_at=? WHERE user_id=? AND episode_id=? AND subscription_id=? AND state='sending'`, at.UTC().Format(time.RFC3339Nano), next, userID, episodeID, subscriptionID)
	return err
}
