package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/afonsocosta/visto/internal/application/plexsync"
)

func (store *Store) IssuePlexWebhook(ctx context.Context, userID, tokenHash string, now time.Time) error {
	_, err := store.DB.ExecContext(ctx, `INSERT INTO plex_webhooks(user_id,token_hash,created_at,last_used_at) VALUES(?,?,?,NULL)
		ON CONFLICT(user_id) DO UPDATE SET token_hash=excluded.token_hash,created_at=excluded.created_at,last_used_at=NULL`,
		userID, tokenHash, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save Plex webhook token: %w", err)
	}
	return nil
}

func (store *Store) RevokePlexWebhook(ctx context.Context, userID string) error {
	_, err := store.DB.ExecContext(ctx, `DELETE FROM plex_webhooks WHERE user_id=?`, userID)
	if err != nil {
		return fmt.Errorf("revoke Plex webhook token: %w", err)
	}
	return nil
}

func (store *Store) GetPlexWebhookStatus(ctx context.Context, userID string) (plexsync.Status, error) {
	status := plexsync.Status{RecentEvents: []plexsync.Event{}}
	var createdAt, lastUsedAt, lastSyncedAt sql.NullString
	err := store.DB.QueryRowContext(ctx, `SELECT created_at,last_used_at FROM plex_webhooks WHERE user_id=?`, userID).Scan(&createdAt, &lastUsedAt)
	if err != nil && err != sql.ErrNoRows {
		return status, fmt.Errorf("get Plex webhook status: %w", err)
	}
	if err == nil {
		status.Enabled = true
		status.CreatedAt = parseNullableTime(createdAt)
		status.LastUsedAt = parseNullableTime(lastUsedAt)
	}
	if err := store.DB.QueryRowContext(ctx, `SELECT MAX(created_at) FROM plex_webhook_events WHERE user_id=? AND status='synced'`, userID).Scan(&lastSyncedAt); err != nil {
		return status, fmt.Errorf("get last successful Plex sync: %w", err)
	}
	status.LastSyncedAt = parseNullableTime(lastSyncedAt)
	rows, err := store.DB.QueryContext(ctx, `SELECT id,COALESCE(tmdb_id,0),status,COALESCE(title,''),COALESCE(media_type,''),COALESCE(message,''),occurred_at
		FROM plex_webhook_events WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT 20`, userID)
	if err != nil {
		return status, fmt.Errorf("list recent Plex sync events: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var event plexsync.Event
		var occurredAt string
		if err := rows.Scan(&event.ID, &event.TMDBID, &event.Status, &event.Title, &event.MediaType, &event.Message, &occurredAt); err != nil {
			return status, fmt.Errorf("scan Plex sync event: %w", err)
		}
		event.OccurredAt, err = time.Parse(time.RFC3339Nano, occurredAt)
		if err != nil {
			return status, fmt.Errorf("parse Plex event time: %w", err)
		}
		status.RecentEvents = append(status.RecentEvents, event)
	}
	if err := rows.Err(); err != nil {
		return status, fmt.Errorf("iterate Plex sync events: %w", err)
	}
	return status, nil
}

func (store *Store) UserForPlexWebhook(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	var userID string
	err := store.DB.QueryRowContext(ctx, `UPDATE plex_webhooks SET last_used_at=? WHERE token_hash=? RETURNING user_id`, now.UTC().Format(time.RFC3339Nano), tokenHash).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("authenticate Plex webhook: %w", err)
	}
	return userID, nil
}

func (store *Store) LogPlexEvent(ctx context.Context, userID, fingerprint string, event plexsync.Event) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Plex event log: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO plex_webhook_events(user_id,fingerprint,status,title,media_type,tmdb_id,message,occurred_at,created_at)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,fingerprint) DO UPDATE SET status=excluded.status,title=excluded.title,media_type=excluded.media_type,tmdb_id=excluded.tmdb_id,message=excluded.message,occurred_at=excluded.occurred_at,created_at=excluded.created_at
		WHERE plex_webhook_events.status='failed'`, userID, fingerprint, event.Status, nullableText(event.Title), nullableText(event.MediaType), nullableInt(event.TMDBID), nullableText(event.Message), event.OccurredAt.UTC().Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save Plex event status: %w", err)
	}
	if err := trimPlexEvents(ctx, tx, userID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Plex event log: %w", err)
	}
	return nil
}

func (store *Store) RecordPlexPlay(ctx context.Context, userID, fingerprint string, event plexsync.Event, mediaID, episodeID *string, watchedAt time.Time, duplicateWindow time.Duration) (bool, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin Plex play transaction: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	var previousStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM plex_webhook_events WHERE user_id=? AND fingerprint=?`, userID, fingerprint).Scan(&previousStatus)
	if err == nil && previousStatus != "failed" {
		return false, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("check duplicate Plex event: %w", err)
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = watchedAt
	}
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `INSERT INTO plex_webhook_events(user_id,fingerprint,status,title,media_type,tmdb_id,message,occurred_at,created_at)
			VALUES(?,?,'processing',?,?,?,?,?,?)`, userID, fingerprint, nullableText(event.Title), nullableText(event.MediaType), nullableInt(event.TMDBID), nullableText(event.Message), event.OccurredAt.UTC().Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE plex_webhook_events SET status='processing',title=?,media_type=?,tmdb_id=?,message=?,occurred_at=?,created_at=? WHERE user_id=? AND fingerprint=?`, nullableText(event.Title), nullableText(event.MediaType), nullableInt(event.TMDBID), nullableText(event.Message), event.OccurredAt.UTC().Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), userID, fingerprint)
	}
	if err != nil {
		return false, fmt.Errorf("claim Plex event: %w", err)
	}
	if (mediaID == nil) == (episodeID == nil) {
		return false, fmt.Errorf("Plex play must identify one movie or episode")
	}
	cutoff := now.Add(-duplicateWindow).Format(time.RFC3339Nano)
	var recent int
	if mediaID != nil {
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND media_id=? AND created_at>=?`, userID, *mediaID, cutoff).Scan(&recent)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND episode_id=? AND created_at>=?`, userID, *episodeID, cutoff).Scan(&recent)
	}
	if err != nil {
		return false, fmt.Errorf("check recent Plex play: %w", err)
	}
	if recent > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE plex_webhook_events SET status='skipped',message=?,created_at=? WHERE user_id=? AND fingerprint=?`, "A recent Visto watch already exists for this item", now.Format(time.RFC3339Nano), userID, fingerprint); err != nil {
			return false, fmt.Errorf("record duplicate Plex play: %w", err)
		}
		if err := trimPlexEvents(ctx, tx, userID); err != nil {
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit duplicate Plex play: %w", err)
		}
		return false, nil
	}
	var priorPlays int
	var trackedMediaID string
	var nullableMediaID, nullableEpisodeID any
	if mediaID != nil {
		trackedMediaID = *mediaID
		nullableMediaID = *mediaID
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND media_id=?`, userID, *mediaID).Scan(&priorPlays)
	} else {
		nullableEpisodeID = *episodeID
		err = tx.QueryRowContext(ctx, `SELECT show_id FROM episodes WHERE id=?`, *episodeID).Scan(&trackedMediaID)
		if err == nil {
			err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plays WHERE user_id=? AND episode_id=?`, userID, *episodeID).Scan(&priorPlays)
		}
	}
	if err != nil {
		return false, fmt.Errorf("load Plex play context: %w", err)
	}
	if err := ensureWatchingRelationship(ctx, tx, userID, trackedMediaID, now); err != nil {
		return false, err
	}
	playResult, err := tx.ExecContext(ctx, `INSERT INTO plays(user_id,media_id,episode_id,watched_at,source,created_at) VALUES(?,?,?,?,'plex',?)`, userID, nullableMediaID, nullableEpisodeID, watchedAt.UTC().Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return false, fmt.Errorf("create Plex play: %w", err)
	}
	playIDValue, err := playResult.LastInsertId()
	if err != nil {
		return false, fmt.Errorf("get Plex play ID: %w", err)
	}
	playID := strconv.FormatInt(playIDValue, 10)
	var visibility string
	if err := tx.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, userID).Scan(&visibility); err != nil {
		return false, fmt.Errorf("get activity visibility: %w", err)
	}
	if visibility == "instance" {
		kind := "watch"
		if priorPlays > 0 {
			kind = "rewatch"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_events(user_id,kind,play_id,media_id,episode_id,occurred_at,created_at) VALUES(?,?,?,?,?,?,?)`, userID, kind, playID, nullableMediaID, nullableEpisodeID, watchedAt.UTC().Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
			return false, fmt.Errorf("create Plex activity event: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE plex_webhook_events SET status='synced',message=?,created_at=? WHERE user_id=? AND fingerprint=?`, "Watched content synchronized", now.Format(time.RFC3339Nano), userID, fingerprint); err != nil {
		return false, fmt.Errorf("mark Plex event synchronized: %w", err)
	}
	if err := trimPlexEvents(ctx, tx, userID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit Plex play: %w", err)
	}
	return true, nil
}

func trimPlexEvents(ctx context.Context, tx *sql.Tx, userID string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM plex_webhook_events WHERE user_id=? AND id NOT IN (SELECT id FROM plex_webhook_events WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT 100)`, userID, userID); err != nil {
		return fmt.Errorf("trim Plex sync history: %w", err)
	}
	return nil
}

func parseNullableTime(value sql.NullString) time.Time {
	if !value.Valid || value.String == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value.String)
	return parsed
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

var _ plexsync.Repository = (*Store)(nil)
