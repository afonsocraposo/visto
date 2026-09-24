package oauth

import (
	"context"
	"testing"
	"time"
)

type cleanupRepository struct {
	now    time.Time
	cutoff time.Time
	limit  int
}

func (repository *cleanupRepository) RegisterOAuthClient(context.Context, Client) error { return nil }
func (repository *cleanupRepository) FindOAuthClient(context.Context, string) (Client, error) {
	return Client{}, ErrInvalidClient
}
func (repository *cleanupRepository) CreateAuthorizationCode(context.Context, AuthorizationCode) error {
	return nil
}
func (repository *cleanupRepository) ExchangeAuthorizationCode(context.Context, string, string, string, string, string, string, string, time.Time, time.Time, time.Time) (Identity, error) {
	return Identity{}, ErrInvalidGrant
}
func (repository *cleanupRepository) RotateOAuthRefreshToken(context.Context, string, string, string, string, string, time.Time, time.Time, time.Time) (Identity, error) {
	return Identity{}, ErrInvalidGrant
}
func (repository *cleanupRepository) FindOAuthAccessToken(context.Context, string, string, time.Time) (Identity, error) {
	return Identity{}, ErrInvalidToken
}
func (repository *cleanupRepository) RevokeOAuthToken(context.Context, string, time.Time) error {
	return nil
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
