package oauth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type cleanupRepository struct {
	now      time.Time
	cutoff   time.Time
	limit    int
	client   Client
	code     AuthorizationCode
	identity Identity
	revoked  string
}

func (repository *cleanupRepository) RegisterOAuthClient(_ context.Context, client Client) error {
	repository.client = client
	return nil
}
func (repository *cleanupRepository) FindOAuthClient(context.Context, string) (Client, error) {
	if repository.client.ID == "" {
		return Client{}, ErrInvalidClient
	}
	return repository.client, nil
}
func (repository *cleanupRepository) CreateAuthorizationCode(_ context.Context, code AuthorizationCode) error {
	repository.code = code
	return nil
}
func (repository *cleanupRepository) ExchangeAuthorizationCode(context.Context, string, string, string, string, string, string, string, time.Time, time.Time, time.Time) (Identity, error) {
	if repository.identity.UserID == "" {
		return Identity{}, ErrInvalidGrant
	}
	return repository.identity, nil
}
func (repository *cleanupRepository) RotateOAuthRefreshToken(context.Context, string, string, string, string, string, time.Time, time.Time, time.Time) (Identity, error) {
	if repository.identity.UserID == "" {
		return Identity{}, ErrInvalidGrant
	}
	return repository.identity, nil
}
func (repository *cleanupRepository) FindOAuthAccessToken(context.Context, string, string, time.Time) (Identity, error) {
	if repository.identity.UserID == "" {
		return Identity{}, ErrInvalidToken
	}
	return repository.identity, nil
}
func (repository *cleanupRepository) RevokeOAuthToken(_ context.Context, token string, _ time.Time) error {
	repository.revoked = token
	return nil
}

func TestRegisterClient_GivenValidAndInvalidRedirects_WhenRegistering_ThenOnlySafeClientsAreSaved(t *testing.T) {
	repository := &cleanupRepository{}
	service := NewService(repository)
	if _, err := service.RegisterClient(context.Background(), "Companion", []string{"http://example.com/callback"}); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("insecure redirect error = %v, want ErrInvalidClient", err)
	}
	client, err := service.RegisterClient(context.Background(), "Companion", []string{"https://companion.example/callback", "http://localhost:3000/callback"})
	if err != nil {
		t.Fatalf("register client: %v", err)
	}
	if client.ID == "" || client.ID != repository.client.ID || len(client.RedirectURIs) != 2 {
		t.Fatalf("saved client = %+v", repository.client)
	}
}

func TestCreateAuthorizationCode_GivenPKCERequest_WhenValid_ThenItStoresAHashedExpiringCode(t *testing.T) {
	repository := &cleanupRepository{client: Client{ID: "client", RedirectURIs: []string{"https://app.example/callback"}}}
	service := NewService(repository)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	challenge := pkceChallenge("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~abc")
	code, err := service.CreateAuthorizationCode(context.Background(), "user", AuthorizeRequest{ClientID: "client", RedirectURI: "https://app.example/callback", Resource: "https://visto.example/mcp", Scopes: []string{ReadScope}, CodeChallenge: challenge, ChallengeMethod: "S256"}, "https://visto.example/mcp")
	if err != nil {
		t.Fatalf("create code: %v", err)
	}
	if code == "" || repository.code.Hash != hashToken(code) || repository.code.ExpiresAt != now.Add(5*time.Minute) || repository.code.UserID != "user" {
		t.Fatalf("stored code = %+v", repository.code)
	}
	_, err = service.CreateAuthorizationCode(context.Background(), "user", AuthorizeRequest{ClientID: "client", RedirectURI: "https://attacker.example/callback", Resource: "https://visto.example/mcp", CodeChallenge: challenge, ChallengeMethod: "S256"}, "https://visto.example/mcp")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unregistered redirect error = %v, want ErrInvalidRequest", err)
	}
}

func TestTokens_GivenAValidIdentity_WhenExchangingRefreshingAuthenticatingAndRevoking_ThenTheServiceProtectsTokenBoundaries(t *testing.T) {
	repository := &cleanupRepository{identity: Identity{UserID: "user", ClientID: "client", Scopes: []string{ReadScope, OfflineScope}}}
	service := NewService(repository)
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~abc"
	issued, err := service.ExchangeCode(context.Background(), "code", "client", "https://app.example/callback", verifier, "https://visto.example/mcp")
	if err != nil || issued.AccessToken == "" || issued.RefreshToken == "" || issued.Scope != "read offline_access" {
		t.Fatalf("exchange = %+v, %v", issued, err)
	}
	refreshed, err := service.Refresh(context.Background(), issued.RefreshToken, "client", "https://visto.example/mcp")
	if err != nil || refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		t.Fatalf("refresh = %+v, %v", refreshed, err)
	}
	identity, err := service.Authenticate(context.Background(), issued.AccessToken, "https://visto.example/mcp")
	if err != nil || identity.UserID != "user" {
		t.Fatalf("authenticate = %+v, %v", identity, err)
	}
	if err := service.Revoke(context.Background(), issued.AccessToken); err != nil || repository.revoked != hashToken(issued.AccessToken) {
		t.Fatalf("revoke = %q, %v", repository.revoked, err)
	}
	if _, err := service.ExchangeCode(context.Background(), "", "client", "", verifier, "https://visto.example/mcp"); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("empty code error = %v, want ErrInvalidGrant", err)
	}
}
func (repository *cleanupRepository) ListOAuthConnections(context.Context, string, time.Time) ([]Connection, error) {
	return nil, nil
}
func (repository *cleanupRepository) RevokeOAuthConnection(context.Context, string, string, time.Time) error {
	return nil
}
func (repository *cleanupRepository) RevokeAllOAuthConnections(context.Context, string, time.Time) error {
	return nil
}
func (repository *cleanupRepository) CleanupOAuthRecords(_ context.Context, now, cutoff time.Time, limit int) (int, error) {
	repository.now, repository.cutoff, repository.limit = now, cutoff, limit
	return 3, nil
}

func TestCleanup_GivenAControlledClock_WhenOAuthRecordsAreCleaned_ThenItUsesTheRetentionBoundaryAndBoundedBatch(t *testing.T) {
	repository := &cleanupRepository{}
	service := NewService(repository)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	removed, err := service.Cleanup(context.Background())
	if err != nil || removed != 3 {
		t.Fatalf("cleanup = %d, %v", removed, err)
	}
	if repository.now != now || repository.cutoff != now.Add(-TokenAuditRetention) || repository.limit != CleanupBatchSize {
		t.Fatalf("cleanup arguments = now:%s cutoff:%s limit:%d", repository.now, repository.cutoff, repository.limit)
	}
}
