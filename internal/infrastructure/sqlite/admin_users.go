package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
)

func (store *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT id,username,display_name,'',role,created_at FROM users ORDER BY created_at,id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := make([]domain.User, 0)
	for rows.Next() {
		user, _, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (store *Store) UpdateUser(ctx context.Context, userID, displayName, passwordHash, keepSessionHash string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET display_name=?,password_hash=CASE WHEN ?='' THEN password_hash ELSE ? END,updated_at=? WHERE id=?`, displayName, passwordHash, passwordHash, time.Now().UTC().Format(time.RFC3339Nano), userID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect updated user: %w", err)
	}
	if changed == 0 {
		return auth.ErrUserNotFound
	}
	if passwordHash != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=? AND token_hash<>?`, userID, keepSessionHash); err != nil {
			return fmt.Errorf("revoke sessions after password change: %w", err)
		}
	}
	return tx.Commit()
}

func (store *Store) DeleteUser(ctx context.Context, userID string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var role domain.Role
	err = tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id=?`, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("load user before deletion: %w", err)
	}
	if role == domain.AdminRole {
		var admins int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&admins); err != nil {
			return err
		}
		if admins <= 1 {
			return auth.ErrLastAdministrator
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id=?`, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return tx.Commit()
}

var _ interface {
	ListUsers(context.Context) ([]domain.User, error)
	UpdateUser(context.Context, string, string, string, string) error
	DeleteUser(context.Context, string) error
} = (*Store)(nil)
