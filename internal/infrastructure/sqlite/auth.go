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

func (store *Store) BootstrapAdmin(ctx context.Context, user domain.User, passwordHash string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return auth.ErrBootstrapComplete
	}
	now := user.CreatedAt.Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, user.ID, user.Username, user.DisplayName, passwordHash, user.Role, now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_settings(user_id,created_at,updated_at) VALUES(?,?,?)`, user.ID, now, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *Store) CreateUser(ctx context.Context, user domain.User, passwordHash string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := user.CreatedAt.Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, user.ID, user.Username, user.DisplayName, passwordHash, user.Role, now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_settings(user_id,created_at,updated_at) VALUES(?,?,?)`, user.ID, now, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *Store) UserCount(ctx context.Context) (int, error) {
	var count int
	if err := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

func (store *Store) FindUserByUsername(ctx context.Context, username string) (domain.User, string, error) {
	row := store.DB.QueryRowContext(ctx, `SELECT id,username,display_name,password_hash,role,created_at FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (store *Store) CreateSession(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := store.DB.ExecContext(ctx, `INSERT INTO sessions(id,user_id,token_hash,expires_at,created_at) VALUES(?,?,?,?,?)`, id, userID, tokenHash, expiresAt.Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (store *Store) FindUserBySessionToken(ctx context.Context, tokenHash string, now time.Time) (domain.User, error) {
	row := store.DB.QueryRowContext(ctx, `SELECT u.id,u.username,u.display_name,u.password_hash,u.role,u.created_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, tokenHash, now.Format(time.RFC3339Nano))
	user, _, err := scanUser(row)
	return user, err
}

type scanner interface{ Scan(...any) error }

func scanUser(row scanner) (domain.User, string, error) {
	var user domain.User
	var passwordHash, createdAt string
	if err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &passwordHash, &user.Role, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, "", auth.ErrInvalidCredentials
		}
		return domain.User{}, "", fmt.Errorf("scan user: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return domain.User{}, "", err
	}
	user.CreatedAt = parsed
	return user, passwordHash, nil
}

var _ auth.Repository = (*Store)(nil)
