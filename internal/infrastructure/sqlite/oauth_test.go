package sqlite_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/oauth"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestOAuth_GivenValidPKCEAuthorization_WhenCodeIsExchangedAndRefreshed_ThenTokensAreHashedScopedRotatedAndRevocable(t *testing.T) {
	store, err := sqlite.Open(context.Background(), t.TempDir()+"/oauth.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	user, err := auth.NewService(store).Bootstrap(context.Background(), "oauth-admin", "OAuth Admin", "a-strong-test-password")
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	service := oauth.NewService(store)
	client, err := service.RegisterClient(context.Background(), "ChatGPT", []string{"https://chatgpt.com/connector_platform_oauth_redirect"})
	if err != nil {
		t.Fatalf("register client: %v", err)
	}
	verifier := strings.Repeat("v", 43)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	resource := "https://visto.example/mcp"
	code, err := service.CreateAuthorizationCode(context.Background(), user.ID, oauth.AuthorizeRequest{ClientID: client.ID, RedirectURI: client.RedirectURIs[0], State: "state-123", Scopes: []string{oauth.ReadScope, oauth.WriteScope, oauth.OfflineScope}, Resource: resource, CodeChallenge: challenge, ChallengeMethod: "S256"}, resource)
	if err != nil {
		t.Fatalf("create authorization code: %v", err)
	}
	tokens, err := service.ExchangeCode(context.Background(), code, client.ID, client.RedirectURIs[0], verifier, resource)
	if err != nil {
		t.Fatalf("exchange authorization code: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.TokenType != "Bearer" {
		t.Fatalf("unexpected token response: %+v", tokens)
	}
	identity, err := service.Authenticate(context.Background(), tokens.AccessToken, resource)
	if err != nil {
		t.Fatalf("authenticate access token: %v", err)
	}
	if identity.UserID != user.ID || !containsScope(identity.Scopes, oauth.ReadScope) || !containsScope(identity.Scopes, oauth.WriteScope) {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	connections, err := service.Connections(context.Background(), user.ID)
	if err != nil || len(connections) != 1 {
		t.Fatalf("connections = %+v, %v; want one connection", connections, err)
	}
	if connections[0].ClientID != client.ID || connections[0].ClientName != "ChatGPT" || !containsScope(connections[0].Scopes, oauth.WriteScope) || connections[0].ConnectedAt == nil || connections[0].LastUsedAt == nil {
		t.Fatalf("unexpected connection: %+v", connections[0])
	}
	if _, err := service.ExchangeCode(context.Background(), code, client.ID, client.RedirectURIs[0], verifier, resource); err == nil {
		t.Fatal("authorization code was reusable")
	}
	refreshed, err := service.Refresh(context.Background(), tokens.RefreshToken, client.ID, resource)
	if err != nil {
		t.Fatalf("refresh access token: %v", err)
	}
	if _, err := service.Refresh(context.Background(), tokens.RefreshToken, client.ID, resource); err == nil {
		t.Fatal("refresh token was reusable after rotation")
	}
	if err := service.Revoke(context.Background(), refreshed.AccessToken); err != nil {
		t.Fatalf("revoke access token: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), refreshed.AccessToken, resource); err == nil {
		t.Fatal("revoked access token remained valid")
	}
	if err := service.RevokeConnection(context.Background(), user.ID, client.ID); err != nil {
		t.Fatalf("revoke connection: %v", err)
	}
	if _, err := service.Refresh(context.Background(), refreshed.RefreshToken, client.ID, resource); err == nil {
		t.Fatal("revoked connection refresh token remained valid")
	}
	connections, err = service.Connections(context.Background(), user.ID)
	if err != nil || len(connections) != 0 {
		t.Fatalf("connections after revocation = %+v, %v; want none", connections, err)
	}
}

func TestOAuth_GivenExpiredAndRevokedRecords_WhenCleaned_ThenOnlyRecordsPastRetentionAreDeletedInBatches(t *testing.T) {
	store, err := sqlite.Open(context.Background(), t.TempDir()+"/oauth-cleanup.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	user, err := auth.NewService(store).Bootstrap(context.Background(), "cleanup-admin", "Cleanup Admin", "a-strong-test-password")
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	service := oauth.NewService(store)
	client, err := service.RegisterClient(context.Background(), "Cleanup Client", []string{"https://example.com/callback"})
	if err != nil {
		t.Fatalf("register client: %v", err)
	}
	now := time.Now().UTC()
	old := now.Add(-oauth.TokenAuditRetention - time.Hour).Format(time.RFC3339Nano)
	recent := now.Add(-time.Hour).Format(time.RFC3339Nano)
	future := now.Add(time.Hour).Format(time.RFC3339Nano)
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO oauth_authorization_codes(code_hash,client_id,user_id,redirect_uri,code_challenge,scope,resource,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, []any{"old-code", client.ID, user.ID, client.RedirectURIs[0], "challenge", "read", "resource", old, old}},
		{`INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,resource,expires_at,created_at,revoked_at) VALUES(?,?,?,?,?,?,?,?)`, []any{"old-access", client.ID, user.ID, "read", "resource", old, old, nil}},
		{`INSERT INTO oauth_refresh_tokens(token_hash,client_id,user_id,scope,resource,expires_at,created_at,revoked_at) VALUES(?,?,?,?,?,?,?,?)`, []any{"old-refresh", client.ID, user.ID, "read", "resource", future, old, old}},
		{`INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,resource,expires_at,created_at,revoked_at) VALUES(?,?,?,?,?,?,?,?)`, []any{"recent-revoked", client.ID, user.ID, "read", "resource", future, recent, recent}},
		{`INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,resource,expires_at,created_at,revoked_at) VALUES(?,?,?,?,?,?,?,?)`, []any{"live", client.ID, user.ID, "read", "resource", future, recent, nil}},
	} {
		if _, err := store.DB.ExecContext(context.Background(), statement.query, statement.args...); err != nil {
			t.Fatalf("insert OAuth fixture: %v", err)
		}
	}

	removed, err := service.Cleanup(context.Background())
	if err != nil || removed != 3 {
		t.Fatalf("cleanup = %d, %v; want three removed records", removed, err)
	}
	for _, token := range []string{"recent-revoked", "live"} {
		var count int
		if err := store.DB.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM oauth_access_tokens WHERE token_hash=?`, token).Scan(&count); err != nil || count != 1 {
			t.Fatalf("token %q was removed unexpectedly: count=%d err=%v", token, count, err)
		}
	}
}

func containsScope(scopes []string, wanted string) bool {
	for _, scope := range scopes {
		if scope == wanted {
			return true
		}
	}
	return false
}
