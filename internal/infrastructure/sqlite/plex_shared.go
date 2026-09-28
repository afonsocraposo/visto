package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/plexsync"
)

func (store *Store) IssueSharedPlexWebhook(ctx context.Context, tokenHash string, now time.Time) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM plex_webhooks WHERE user_id IS NULL`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO plex_webhooks(user_id,token_hash,account_id,created_at,last_used_at) VALUES(NULL,?,'',?,NULL)`, tokenHash, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *Store) RevokeSharedPlexWebhook(ctx context.Context) error {
	_, err := store.DB.ExecContext(ctx, `DELETE FROM plex_webhooks WHERE user_id IS NULL`)
	return err
}

func (store *Store) SharedPlexWebhookStatus(ctx context.Context) (bool, time.Time, time.Time, error) {
	var created, used sql.NullString
	err := store.DB.QueryRowContext(ctx, `SELECT created_at,last_used_at FROM plex_webhooks WHERE user_id IS NULL`).Scan(&created, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return false, time.Time{}, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, time.Time{}, err
	}
	return true, parseNullableTime(created), parseNullableTime(used), nil
}

func (store *Store) SharedPlexWebhookByToken(ctx context.Context, hash string, now time.Time) (bool, error) {
	var id int
	err := store.DB.QueryRowContext(ctx, `UPDATE plex_webhooks SET last_used_at=? WHERE token_hash=? AND user_id IS NULL RETURNING id`, now.UTC().Format(time.RFC3339Nano), hash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (store *Store) PlexUserForAccount(ctx context.Context, accountID string) (string, error) {
	var userID string
	err := store.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE plex_account_id=?`, accountID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", plexsync.ErrAccountUnmapped
	}
	return userID, err
}

func (store *Store) ObservePlexAccount(ctx context.Context, accountID, title string, now time.Time) error {
	title = strings.TrimSpace(title)
	if len(title) > 100 {
		title = title[:100]
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO plex_observed_accounts(account_id,title,last_seen_at) VALUES(?,?,?)
		ON CONFLICT(account_id) DO UPDATE SET title=CASE WHEN excluded.title='' THEN plex_observed_accounts.title ELSE excluded.title END,last_seen_at=excluded.last_seen_at`, accountID, title, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM plex_observed_accounts WHERE account_id NOT IN (SELECT account_id FROM plex_observed_accounts ORDER BY last_seen_at DESC,account_id DESC LIMIT 100)`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (store *Store) ListPlexAccounts(ctx context.Context) ([]plexsync.ObservedAccount, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT o.account_id,o.title,o.last_seen_at,COALESCE(u.id,'')
		FROM plex_observed_accounts o LEFT JOIN users u ON u.plex_account_id=o.account_id
		ORDER BY o.last_seen_at DESC,o.account_id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := []plexsync.ObservedAccount{}
	for rows.Next() {
		var account plexsync.ObservedAccount
		var seen string
		if err := rows.Scan(&account.AccountID, &account.Title, &seen, &account.UserID); err != nil {
			return nil, err
		}
		account.LastSeenAt, err = time.Parse(time.RFC3339Nano, seen)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (store *Store) ListPlexMappings(ctx context.Context) ([]plexsync.Mapping, error) {
	rows, err := store.DB.QueryContext(ctx, `SELECT id,plex_account_id FROM users WHERE plex_account_id IS NOT NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	mappings := []plexsync.Mapping{}
	for rows.Next() {
		var mapping plexsync.Mapping
		if err := rows.Scan(&mapping.UserID, &mapping.AccountID); err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}
	return mappings, rows.Err()
}

func (store *Store) SetPlexMapping(ctx context.Context, userID, accountID string) error {
	result, err := store.DB.ExecContext(ctx, `UPDATE users SET plex_account_id=? WHERE id=?`, accountID, userID)
	if err != nil {
		return fmt.Errorf("save Plex account mapping: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("Visto user not found")
	}
	return nil
}

func (store *Store) DeletePlexMapping(ctx context.Context, userID string) error {
	_, err := store.DB.ExecContext(ctx, `UPDATE users SET plex_account_id=NULL WHERE id=?`, userID)
	return err
}

func (store *Store) PlexMappingForUser(ctx context.Context, userID string) (string, error) {
	var accountID string
	err := store.DB.QueryRowContext(ctx, `SELECT COALESCE(plex_account_id,'') FROM users WHERE id=?`, userID).Scan(&accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return accountID, err
}
