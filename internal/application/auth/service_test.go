package auth

import (
	"context"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

type createUserRepository struct {
	createdUser  domain.User
	passwordHash string
}

func (*createUserRepository) BootstrapAdmin(context.Context, domain.User, string) error { return nil }
func (repository *createUserRepository) CreateUser(_ context.Context, user domain.User, passwordHash string) error {
	repository.createdUser = user
	repository.passwordHash = passwordHash
	return nil
}
func (*createUserRepository) FindUserByUsername(context.Context, string) (domain.User, string, error) {
	return domain.User{}, "", nil
}
func (*createUserRepository) CreateSession(context.Context, string, string, string, time.Time) error {
	return nil
}
func (*createUserRepository) FindUserBySessionToken(context.Context, string, time.Time) (domain.User, error) {
	return domain.User{}, nil
}
func (*createUserRepository) RevokeSession(context.Context, string) error { return nil }

func TestCreateUser_GivenValidAccount_WhenCreatedByAdministrator_ThenItStoresARegularUserWithHashedPassword(t *testing.T) {
	repository := &createUserRepository{}
	service := NewService(repository)
	user, err := service.CreateUser(context.Background(), " family ", " Family Member ", "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.ID == "" || user.Username != "family" || user.DisplayName != "Family Member" || user.Role != domain.UserRole {
		t.Fatalf("created user = %+v", user)
	}
	if repository.createdUser.ID != user.ID || repository.createdUser.Role != domain.UserRole {
		t.Fatalf("stored user = %+v", repository.createdUser)
	}
	if repository.passwordHash == "" || repository.passwordHash == "correct-horse-battery-staple" || !verifyPassword(repository.passwordHash, "correct-horse-battery-staple") {
		t.Fatal("password must be stored as a verifiable hash, never as plaintext")
	}
}

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
