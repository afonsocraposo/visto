package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/profile"
)

func (store *Store) GetSettings(ctx context.Context, userID string) (profile.Settings, error) {
	var settings profile.Settings
	err := store.DB.QueryRowContext(ctx, `SELECT activity_visibility,timezone FROM user_settings WHERE user_id=?`, userID).Scan(&settings.ActivityVisibility, &settings.Timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return profile.Settings{}, fmt.Errorf("settings not found")
	}
	if err != nil {
		return profile.Settings{}, fmt.Errorf("get settings: %w", err)
	}
	return settings, nil
}

func (store *Store) SetSettings(ctx context.Context, userID string, settings profile.Settings) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE user_settings SET activity_visibility=?,timezone=?,updated_at=? WHERE user_id=?`, settings.ActivityVisibility, settings.Timezone, time.Now().UTC().Format(time.RFC3339Nano), userID)
	if err != nil {
		return fmt.Errorf("set activity visibility: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect activity visibility update: %w", err)
	}
	if changed == 0 {
		return fmt.Errorf("settings not found")
	}
	if settings.ActivityVisibility == profile.PrivateVisibility {
		if _, err := tx.ExecContext(ctx, `DELETE FROM activity_events WHERE user_id=?`, userID); err != nil {
			return fmt.Errorf("remove private activity: %w", err)
		}
	}
	return tx.Commit()
}

func (store *Store) GetPushoverSettings(ctx context.Context, userID string) (enabled, hasKey bool, err error) {
	err = store.DB.QueryRowContext(ctx, `SELECT pushover_notifications_enabled,pushover_user_key_encrypted IS NOT NULL FROM user_settings WHERE user_id=?`, userID).Scan(&enabled, &hasKey)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, fmt.Errorf("settings not found")
	}
	if err != nil {
		return false, false, fmt.Errorf("get Pushover settings: %w", err)
	}
	return enabled, hasKey, nil
}

func (store *Store) SetPushoverSettings(ctx context.Context, userID string, encryptedKey *string, enabled bool) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var wasEnabled bool
	if err := tx.QueryRowContext(ctx, `SELECT pushover_notifications_enabled FROM user_settings WHERE user_id=?`, userID).Scan(&wasEnabled); err != nil {
		return fmt.Errorf("read Pushover settings: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `UPDATE user_settings SET
		pushover_user_key_encrypted=COALESCE(?,pushover_user_key_encrypted),pushover_notifications_enabled=?,updated_at=?
		WHERE user_id=?`, encryptedKey, enabled, now, userID)
	if err != nil {
		return fmt.Errorf("save Pushover settings: %w", err)
	}
	if enabled && !wasEnabled {
		if _, err := tx.ExecContext(ctx, `UPDATE user_media SET notifications_since=? WHERE user_id=? AND status='watching'`, now, userID); err != nil {
			return fmt.Errorf("set Pushover notification baseline: %w", err)
		}
	}
	return tx.Commit()
}

func (store *Store) ClearPushoverKey(ctx context.Context, userID string) error {
	result, err := store.DB.ExecContext(ctx, `UPDATE user_settings SET pushover_user_key_encrypted=NULL,pushover_notifications_enabled=0,updated_at=? WHERE user_id=?`, time.Now().UTC().Format(time.RFC3339Nano), userID)
	if err != nil {
		return fmt.Errorf("clear Pushover user key: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect cleared Pushover settings: %w", err)
	}
	if changed == 0 {
		return fmt.Errorf("settings not found")
	}
	return nil
}

var _ profile.Repository = (*Store)(nil)
