package sqlite_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

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
}

func containsScope(scopes []string, wanted string) bool {
	for _, scope := range scopes {
		if scope == wanted {
			return true
		}
	}
	return false
}
