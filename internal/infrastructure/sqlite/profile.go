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
	err := store.DB.QueryRowContext(ctx, `SELECT activity_visibility FROM user_settings WHERE user_id=?`, userID).Scan(&settings.ActivityVisibility)
	if errors.Is(err, sql.ErrNoRows) {
		return profile.Settings{}, fmt.Errorf("settings not found")
	}
	if err != nil {
		return profile.Settings{}, fmt.Errorf("get settings: %w", err)
	}
	return settings, nil
}

func (store *Store) SetActivityVisibility(ctx context.Context, userID, visibility string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE user_settings SET activity_visibility=?,updated_at=? WHERE user_id=?`, visibility, time.Now().UTC().Format(time.RFC3339Nano), userID)
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
	if visibility == profile.PrivateVisibility {
		if _, err := tx.ExecContext(ctx, `DELETE FROM activity_events WHERE user_id=?`, userID); err != nil {
			return fmt.Errorf("remove private activity: %w", err)
		}
	}
	return tx.Commit()
}

var _ profile.Repository = (*Store)(nil)
