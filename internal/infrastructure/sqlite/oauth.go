package sqlite

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/oauth"
)

func (store *Store) RegisterOAuthClient(ctx context.Context, client oauth.Client) error {
	redirectURIs, err := json.Marshal(client.RedirectURIs)
	if err != nil {
		return err
	}
	_, err = store.DB.ExecContext(ctx, `INSERT INTO oauth_clients(client_id,client_name,redirect_uris_json,created_at) VALUES(?,?,?,?)`, client.ID, client.Name, string(redirectURIs), client.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("register OAuth client: %w", err)
	}
	return nil
}

func (store *Store) FindOAuthClient(ctx context.Context, clientID string) (oauth.Client, error) {
	var client oauth.Client
	var redirects, createdAt string
	err := store.DB.QueryRowContext(ctx, `SELECT client_id,client_name,redirect_uris_json,created_at FROM oauth_clients WHERE client_id=?`, clientID).Scan(&client.ID, &client.Name, &redirects, &createdAt)
	if err != nil {
		return oauth.Client{}, err
	}
	if err := json.Unmarshal([]byte(redirects), &client.RedirectURIs); err != nil {
		return oauth.Client{}, fmt.Errorf("decode OAuth redirects: %w", err)
	}
	client.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return oauth.Client{}, fmt.Errorf("parse OAuth client creation time: %w", err)
	}
	return client, nil
}

func (store *Store) CreateAuthorizationCode(ctx context.Context, code oauth.AuthorizationCode) error {
	_, err := store.DB.ExecContext(ctx, `INSERT INTO oauth_authorization_codes(code_hash,client_id,user_id,redirect_uri,code_challenge,scope,resource,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, code.Hash, code.ClientID, code.UserID, code.RedirectURI, code.Challenge, strings.Join(code.Scopes, " "), code.Resource, stamp(code.ExpiresAt), stamp(code.CreatedAt))
	if err != nil {
		return fmt.Errorf("create OAuth authorization code: %w", err)
	}
	return nil
}

func (store *Store) ExchangeAuthorizationCode(ctx context.Context, codeHash, clientID, redirectURI, challenge, resource, accessHash, refreshHash string, now, accessExpiry, refreshExpiry time.Time) (oauth.Identity, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return oauth.Identity{}, fmt.Errorf("begin OAuth code exchange: %w", err)
	}
	defer tx.Rollback()
	var identity oauth.Identity
	var storedRedirect, storedChallenge, scopes, storedResource, expiresAt string
	err = tx.QueryRowContext(ctx, `SELECT user_id,client_id,redirect_uri,code_challenge,scope,resource,expires_at FROM oauth_authorization_codes WHERE code_hash=? AND client_id=?`, codeHash, clientID).Scan(&identity.UserID, &identity.ClientID, &storedRedirect, &storedChallenge, &scopes, &storedResource, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return oauth.Identity{}, oauth.ErrInvalidGrant
	}
	if err != nil {
		return oauth.Identity{}, fmt.Errorf("read OAuth code: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_authorization_codes WHERE code_hash=?`, codeHash); err != nil {
		return oauth.Identity{}, fmt.Errorf("consume OAuth code: %w", err)
	}
	expiry, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return oauth.Identity{}, fmt.Errorf("parse OAuth code expiry: %w", err)
	}
	if storedRedirect != redirectURI || storedResource != resource || !expiry.After(now.UTC()) || !constantTimeEqual(storedChallenge, challenge) {
		if err := tx.Commit(); err != nil {
			return oauth.Identity{}, err
		}
		return oauth.Identity{}, oauth.ErrInvalidGrant
	}
	identity.Scopes = strings.Fields(scopes)
	identity.Resource = storedResource
	identity.Expires = accessExpiry.UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,resource,expires_at) VALUES(?,?,?,?,?,?)`, accessHash, identity.ClientID, identity.UserID, scopes, resource, stamp(accessExpiry)); err != nil {
		return oauth.Identity{}, fmt.Errorf("save OAuth access token: %w", err)
	}
	if containsString(identity.Scopes, oauth.OfflineScope) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO oauth_refresh_tokens(token_hash,client_id,user_id,scope,resource,expires_at) VALUES(?,?,?,?,?,?)`, refreshHash, identity.ClientID, identity.UserID, scopes, resource, stamp(refreshExpiry)); err != nil {
			return oauth.Identity{}, fmt.Errorf("save OAuth refresh token: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return oauth.Identity{}, fmt.Errorf("commit OAuth code exchange: %w", err)
	}
	return identity, nil
}

func (store *Store) RotateOAuthRefreshToken(ctx context.Context, oldHash, clientID, resource, accessHash, refreshHash string, now, accessExpiry, refreshExpiry time.Time) (oauth.Identity, error) {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return oauth.Identity{}, fmt.Errorf("begin OAuth refresh: %w", err)
	}
	defer tx.Rollback()
	var identity oauth.Identity
	var scopes, storedResource, expiresAt string
	err = tx.QueryRowContext(ctx, `SELECT user_id,client_id,scope,resource,expires_at FROM oauth_refresh_tokens WHERE token_hash=? AND client_id=? AND revoked_at IS NULL`, oldHash, clientID).Scan(&identity.UserID, &identity.ClientID, &scopes, &storedResource, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return oauth.Identity{}, oauth.ErrInvalidGrant
	}
	if err != nil {
		return oauth.Identity{}, fmt.Errorf("read OAuth refresh token: %w", err)
	}
	expiry, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return oauth.Identity{}, fmt.Errorf("parse OAuth refresh expiry: %w", err)
	}
	if storedResource != resource || !expiry.After(now.UTC()) {
		return oauth.Identity{}, oauth.ErrInvalidGrant
	}
	result, err := tx.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked_at=? WHERE token_hash=? AND revoked_at IS NULL`, stamp(now), oldHash)
	if err != nil {
		return oauth.Identity{}, fmt.Errorf("rotate OAuth refresh token: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return oauth.Identity{}, oauth.ErrInvalidGrant
	}
	identity.Scopes, identity.Resource, identity.Expires = strings.Fields(scopes), storedResource, accessExpiry.UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,resource,expires_at) VALUES(?,?,?,?,?,?)`, accessHash, identity.ClientID, identity.UserID, scopes, resource, stamp(accessExpiry)); err != nil {
		return oauth.Identity{}, fmt.Errorf("save refreshed access token: %w", err)
	}
	if containsString(identity.Scopes, oauth.OfflineScope) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO oauth_refresh_tokens(token_hash,client_id,user_id,scope,resource,expires_at) VALUES(?,?,?,?,?,?)`, refreshHash, identity.ClientID, identity.UserID, scopes, resource, stamp(refreshExpiry)); err != nil {
			return oauth.Identity{}, fmt.Errorf("save rotated refresh token: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return oauth.Identity{}, fmt.Errorf("commit OAuth refresh: %w", err)
	}
	return identity, nil
}

func (store *Store) FindOAuthAccessToken(ctx context.Context, tokenHash, resource string, now time.Time) (oauth.Identity, error) {
	var identity oauth.Identity
	var scopes, expiresAt string
	err := store.DB.QueryRowContext(ctx, `SELECT user_id,client_id,scope,resource,expires_at FROM oauth_access_tokens WHERE token_hash=? AND resource=? AND revoked_at IS NULL AND expires_at>?`, tokenHash, resource, stamp(now)).Scan(&identity.UserID, &identity.ClientID, &scopes, &identity.Resource, &expiresAt)
	if err != nil {
		return oauth.Identity{}, err
	}
	identity.Scopes = strings.Fields(scopes)
	identity.Expires, err = time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return oauth.Identity{}, fmt.Errorf("parse OAuth access expiry: %w", err)
	}
	return identity, nil
}

func (store *Store) RevokeOAuthToken(ctx context.Context, tokenHash string, now time.Time) error {
	if _, err := store.DB.ExecContext(ctx, `UPDATE oauth_access_tokens SET revoked_at=COALESCE(revoked_at,?) WHERE token_hash=?`, stamp(now), tokenHash); err != nil {
		return fmt.Errorf("revoke OAuth access token: %w", err)
	}
	if _, err := store.DB.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked_at=COALESCE(revoked_at,?) WHERE token_hash=?`, stamp(now), tokenHash); err != nil {
		return fmt.Errorf("revoke OAuth refresh token: %w", err)
	}
	return nil
}

func stamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func containsString(values []string, item string) bool {
	for _, value := range values {
		if value == item {
			return true
		}
	}
	return false
}
