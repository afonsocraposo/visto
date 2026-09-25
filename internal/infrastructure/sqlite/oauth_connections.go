package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/oauth"
)

func (store *Store) ListOAuthConnections(ctx context.Context, userID string, now time.Time) ([]oauth.Connection, error) {
	rows, err := store.DB.QueryContext(ctx, `
		SELECT c.client_id, c.client_name, a.scope, a.created_at, a.last_used_at, a.expires_at
		FROM oauth_access_tokens a
		JOIN oauth_clients c ON c.client_id = a.client_id
		WHERE a.user_id=? AND a.revoked_at IS NULL AND a.expires_at>?
		ORDER BY c.client_name, a.created_at`, userID, stamp(now))
	if err != nil {
		return nil, fmt.Errorf("list OAuth connections: %w", err)
	}
	defer rows.Close()
	connections := map[string]*oauth.Connection{}
	order := []string{}
	for rows.Next() {
		var clientID, clientName, scope, createdAt, expiresAt string
		var lastUsed sql.NullString
		if err := rows.Scan(&clientID, &clientName, &scope, &createdAt, &lastUsed, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan OAuth connection: %w", err)
		}
		expires, err := time.Parse(time.RFC3339Nano, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("parse OAuth connection expiry: %w", err)
		}
		connection := connections[clientID]
		if connection == nil {
			connection = &oauth.Connection{ClientID: clientID, ClientName: clientName, ExpiresAt: expires}
			if createdAt != "" {
				if connectedAt, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
					connection.ConnectedAt = &connectedAt
				}
			}
			connections[clientID] = connection
			order = append(order, clientID)
		}
		for _, value := range strings.Fields(scope) {
			if !containsString(connection.Scopes, value) {
				connection.Scopes = append(connection.Scopes, value)
			}
		}
		if expires.After(connection.ExpiresAt) {
			connection.ExpiresAt = expires
		}
		if lastUsed.Valid {
			if used, err := time.Parse(time.RFC3339Nano, lastUsed.String); err == nil && (connection.LastUsedAt == nil || used.After(*connection.LastUsedAt)) {
				connection.LastUsedAt = &used
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate OAuth connections: %w", err)
	}
	result := make([]oauth.Connection, 0, len(order))
	for _, clientID := range order {
		connection := connections[clientID]
		sort.Strings(connection.Scopes)
		result = append(result, *connection)
	}
	return result, nil
}

func (store *Store) RevokeOAuthConnection(ctx context.Context, userID, clientID string, now time.Time) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin OAuth connection revocation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_access_tokens SET revoked_at=COALESCE(revoked_at,?) WHERE user_id=? AND client_id=?`, stamp(now), userID, clientID); err != nil {
		return fmt.Errorf("revoke OAuth access tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked_at=COALESCE(revoked_at,?) WHERE user_id=? AND client_id=?`, stamp(now), userID, clientID); err != nil {
		return fmt.Errorf("revoke OAuth refresh tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_authorization_codes WHERE user_id=? AND client_id=?`, userID, clientID); err != nil {
		return fmt.Errorf("revoke OAuth authorization codes: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit OAuth connection revocation: %w", err)
	}
	return nil
}

func (store *Store) RevokeAllOAuthConnections(ctx context.Context, userID string, now time.Time) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin OAuth connection revocation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_access_tokens SET revoked_at=COALESCE(revoked_at,?) WHERE user_id=?`, stamp(now), userID); err != nil {
		return fmt.Errorf("revoke OAuth access tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked_at=COALESCE(revoked_at,?) WHERE user_id=?`, stamp(now), userID); err != nil {
		return fmt.Errorf("revoke OAuth refresh tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_authorization_codes WHERE user_id=?`, userID); err != nil {
		return fmt.Errorf("revoke OAuth authorization codes: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit OAuth connection revocation: %w", err)
	}
	return nil
}

func (store *Store) CleanupOAuthRecords(ctx context.Context, now, tokenRetentionCutoff time.Time, limit int) (int, error) {
	if limit < 1 {
		return 0, nil
	}
	removed := 0
	deleteBatch := func(query string, args ...any) error {
		result, err := store.DB.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		removed += int(count)
		return nil
	}
	if err := deleteBatch(`DELETE FROM oauth_authorization_codes WHERE rowid IN (SELECT rowid FROM oauth_authorization_codes WHERE expires_at<=? ORDER BY expires_at LIMIT ?)`, stamp(now), limit); err != nil {
		return removed, fmt.Errorf("clean expired OAuth codes: %w", err)
	}
	remaining := limit - removed
	if remaining <= 0 {
		return removed, nil
	}
	if err := deleteBatch(`DELETE FROM oauth_access_tokens WHERE rowid IN (SELECT rowid FROM oauth_access_tokens WHERE expires_at<=? OR revoked_at IS NOT NULL AND revoked_at<=? ORDER BY expires_at LIMIT ?)`, stamp(tokenRetentionCutoff), stamp(tokenRetentionCutoff), remaining); err != nil {
		return removed, fmt.Errorf("clean OAuth access tokens: %w", err)
	}
	remaining = limit - removed
	if remaining <= 0 {
		return removed, nil
	}
	if err := deleteBatch(`DELETE FROM oauth_refresh_tokens WHERE rowid IN (SELECT rowid FROM oauth_refresh_tokens WHERE expires_at<=? OR revoked_at IS NOT NULL AND revoked_at<=? ORDER BY expires_at LIMIT ?)`, stamp(tokenRetentionCutoff), stamp(tokenRetentionCutoff), remaining); err != nil {
		return removed, fmt.Errorf("clean OAuth refresh tokens: %w", err)
	}
	return removed, nil
}

func containsString(values []string, item string) bool {
	for _, value := range values {
		if value == item {
			return true
		}
	}
	return false
}
