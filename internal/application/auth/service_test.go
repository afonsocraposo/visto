package auth

import (
	"context"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

type logoutRepository struct {
	revokedTokenHash string
}

func (repository *logoutRepository) BootstrapAdmin(context.Context, domain.User, string) error {
	return nil
}
func (repository *logoutRepository) CreateUser(context.Context, domain.User, string) error {
	return nil
}
func (repository *logoutRepository) FindUserByUsername(context.Context, string) (domain.User, string, error) {
	return domain.User{}, "", nil
}
func (repository *logoutRepository) CreateSession(context.Context, string, string, string, time.Time) error {
	return nil
}
func (repository *logoutRepository) FindUserBySessionToken(context.Context, string, time.Time) (domain.User, error) {
	return domain.User{}, nil
}
func (repository *logoutRepository) RevokeSession(_ context.Context, tokenHash string) error {
	repository.revokedTokenHash = tokenHash
	return nil
}

func TestLogoutBDD(t *testing.T) {
	t.Run("Given an active session, When the user signs out, Then only the token hash is revoked", func(t *testing.T) {
		repository := &logoutRepository{}
		service := NewService(repository)

		if err := service.Logout(context.Background(), "session-secret"); err != nil {
			t.Fatalf("sign out: %v", err)
		}
		if repository.revokedTokenHash != hashToken("session-secret") {
			t.Fatalf("expected hashed token to be revoked, got %q", repository.revokedTokenHash)
		}
		if repository.revokedTokenHash == "session-secret" {
			t.Fatal("session token must not be stored in plaintext")
		}
	})

	t.Run("Given no session token, When logout is requested, Then revocation is a no-op", func(t *testing.T) {
		repository := &logoutRepository{}
		service := NewService(repository)
		if err := service.Logout(context.Background(), ""); err != nil {
			t.Fatalf("logout without a session: %v", err)
		}
		if repository.revokedTokenHash != "" {
			t.Fatal("expected no token to be revoked")
		}
	})
}
