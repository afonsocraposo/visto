package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
)

func (store *Store) BootstrapAdmin(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&count); err != nil {
		return domain.User{}, err
	}
	if count != 0 {
		return domain.User{}, auth.ErrBootstrapComplete
	}
	user, err = insertUserAndSettings(ctx, tx, user, passwordHash, "")
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (store *Store) CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	user, err = insertUserAndSettings(ctx, tx, user, passwordHash, "")
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (store *Store) CreateSignupUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	var admins int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&admins); err != nil {
		return domain.User{}, err
	}
	if admins == 0 {
		return domain.User{}, auth.ErrBootstrapIncomplete
	}
	user, err = insertUserAndSettings(ctx, tx, user, passwordHash, "")
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func insertUserAndSettings(ctx context.Context, tx *sql.Tx, user domain.User, passwordHash, googleSubject string) (domain.User, error) {
	now := user.CreatedAt.Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `INSERT INTO users(username,display_name,password_hash,role,created_at,updated_at,email,google_subject) VALUES(?,?,?,?,?,?,?,NULLIF(?,'') )`, user.Email, user.DisplayName, passwordHash, user.Role, now, now, user.Email, googleSubject)
	if err != nil {
		return domain.User{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.User{}, fmt.Errorf("get new user ID: %w", err)
	}
	user.ID = strconv.FormatInt(id, 10)
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_settings(user_id,created_at,updated_at) VALUES(?,?,?)`, user.ID, now, now); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (store *Store) AdminCount(ctx context.Context) (int, error) {
	var count int
	if err := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count administrators: %w", err)
	}
	return count, nil
}

func (store *Store) FindUserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	row := store.DB.QueryRowContext(ctx, `SELECT id,email,display_name,password_hash,role,created_at FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func (store *Store) FindUserByGoogleSubject(ctx context.Context, subject string) (domain.User, error) {
	user, _, err := scanUser(store.DB.QueryRowContext(ctx, `SELECT id,email,display_name,password_hash,role,created_at FROM users WHERE google_subject=?`, subject))
	return user, err
}

func (store *Store) LinkGoogleSubject(ctx context.Context, userID, subject string) error {
	result, err := store.DB.ExecContext(ctx, `UPDATE users SET google_subject=?,updated_at=? WHERE id=? AND google_subject IS NULL`, subject, time.Now().UTC().Format(time.RFC3339Nano), userID)
	if err != nil {
		return fmt.Errorf("link Google account: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check Google account link: %w", err)
	}
	if updated == 0 {
		return fmt.Errorf("Google account cannot be linked to this user")
	}
	return nil
}

func (store *Store) CreateGoogleUser(ctx context.Context, user domain.User, subject string) (domain.User, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	var admins int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&admins); err != nil {
		return domain.User{}, err
	}
	if admins == 0 {
		return domain.User{}, auth.ErrBootstrapIncomplete
	}
	user, err = insertUserAndSettings(ctx, tx, user, "", subject)
	if err != nil {
		return domain.User{}, fmt.Errorf("create Google account: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (store *Store) CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := store.DB.ExecContext(ctx, `INSERT INTO sessions(user_id,token_hash,expires_at,created_at) VALUES(?,?,?,?)`, userID, tokenHash, expiresAt.Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (store *Store) FindUserBySessionToken(ctx context.Context, tokenHash string, now time.Time) (domain.User, error) {
	row := store.DB.QueryRowContext(ctx, `SELECT u.id,u.email,u.display_name,u.password_hash,u.role,u.created_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, tokenHash, now.Format(time.RFC3339Nano))
	user, _, err := scanUser(row)
	return user, err
}

func (store *Store) RevokeSession(ctx context.Context, tokenHash string) error {
	if _, err := store.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, tokenHash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (store *Store) CreatePersonalToken(ctx context.Context, userID, name, tokenHash string, createdAt time.Time, expiresAt *time.Time) (string, error) {
	var expiry any
	if expiresAt != nil {
		expiry = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	result, err := store.DB.ExecContext(ctx, `INSERT INTO personal_api_tokens(user_id,name,token_hash,created_at,expires_at) VALUES(?,?,?,?,?)`, userID, name, tokenHash, createdAt.UTC().Format(time.RFC3339Nano), expiry)
	if err != nil {
		return "", fmt.Errorf("create personal API token: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("get new personal API token ID: %w", err)
	}
	return strconv.FormatInt(id, 10), nil
}

func (store *Store) ListPersonalTokens(ctx context.Context, userID string) ([]auth.PersonalToken, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT id,name,created_at,last_used_at,expires_at FROM personal_api_tokens WHERE user_id=? ORDER BY created_at DESC,id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list personal API tokens: %w", err)
	}
	defer rows.Close()
	tokens := []auth.PersonalToken{}
	for rows.Next() {
		var token auth.PersonalToken
		var createdAt string
		var lastUsedAt, expiresAt sql.NullString
		if err := rows.Scan(&token.ID, &token.Name, &createdAt, &lastUsedAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan personal API token: %w", err)
		}
		token.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse personal API token creation time: %w", err)
		}
		if lastUsedAt.Valid {
			parsed, err := time.Parse(time.RFC3339Nano, lastUsedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse personal API token last-used time: %w", err)
			}
			token.LastUsedAt = &parsed
		}
		if expiresAt.Valid {
			parsed, err := time.Parse(time.RFC3339Nano, expiresAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse personal API token expiry: %w", err)
			}
			token.ExpiresAt = &parsed
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate personal API tokens: %w", err)
	}
	return tokens, nil
}

func (store *Store) RevokePersonalToken(ctx context.Context, userID, tokenID string) error {
	result, err := store.DB.ExecContext(ctx, `DELETE FROM personal_api_tokens WHERE id=? AND user_id=?`, tokenID, userID)
	if err != nil {
		return fmt.Errorf("revoke personal API token: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect revoked personal API token: %w", err)
	}
	if deleted == 0 {
		return auth.ErrPersonalTokenMissing
	}
	return nil
}

func (store *Store) FindUserByPersonalTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.User, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin personal API token authentication: %w", err)
	}
	defer tx.Rollback()
	var userID string
	err = tx.QueryRowContext(ctx, `UPDATE personal_api_tokens SET last_used_at=? WHERE token_hash=? AND (expires_at IS NULL OR expires_at>?) RETURNING user_id`, now.UTC().Format(time.RFC3339Nano), tokenHash, now.UTC().Format(time.RFC3339Nano)).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, auth.ErrInvalidCredentials
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("authenticate personal API token: %w", err)
	}
	user, _, err := scanUser(tx.QueryRowContext(ctx, `SELECT id,email,display_name,password_hash,role,created_at FROM users WHERE id=?`, userID))
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, fmt.Errorf("record personal API token use: %w", err)
	}
	return user, nil
}

type scanner interface{ Scan(...any) error }

func scanUser(row scanner) (domain.User, string, error) {
	var user domain.User
	var passwordHash, createdAt string
	if err := row.Scan(&user.ID, &user.Email, &user.DisplayName, &passwordHash, &user.Role, &createdAt); err != nil {
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
