package auth

import (
	"context"
	"errors"
	"strings"
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
func (repository *createUserRepository) CreateSignupUser(_ context.Context, user domain.User, passwordHash string) error {
	repository.createdUser, repository.passwordHash = user, passwordHash
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

func TestSignUp_GivenPublicSignupEnabled_WhenAccountIsCreated_ThenItCreatesARegularUserAndSession(t *testing.T) {
	repository := &createUserRepository{}
	service := NewService(repository)
	user, token, expiresAt, err := service.SignUp(context.Background(), " family ", "correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != domain.UserRole || user.Username != "family" || user.DisplayName != "family" || token == "" || !expiresAt.After(time.Now()) {
		t.Fatalf("signup result = user:%+v token:%q expires:%v", user, token, expiresAt)
	}
	if repository.createdUser.ID != user.ID || !verifyPassword(repository.passwordHash, "correct-horse-battery-staple") {
		t.Fatal("signup did not store the regular user with a password hash")
	}
}

func TestSignUp_GivenPublicSignupDisabled_WhenAccountIsCreated_ThenItIsRejected(t *testing.T) {
	service := NewService(&createUserRepository{}, Config{AllowSignups: false})
	if _, _, _, err := service.SignUp(context.Background(), "family", "correct-horse-battery-staple"); !errors.Is(err, ErrSignupsDisabled) {
		t.Fatalf("signup error = %v, want ErrSignupsDisabled", err)
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

type personalTokenTestRepository struct {
	logoutRepository
	user   domain.User
	tokens map[string]PersonalToken
	hashes map[string]string
}

func (repository *personalTokenTestRepository) CreatePersonalToken(_ context.Context, id, userID, name, tokenHash string, createdAt time.Time, expiresAt *time.Time) error {
	if repository.tokens == nil {
		repository.tokens = map[string]PersonalToken{}
		repository.hashes = map[string]string{}
	}
	item := PersonalToken{ID: id, Name: name, CreatedAt: createdAt, ExpiresAt: expiresAt}
	repository.tokens[id] = item
	repository.hashes[tokenHash] = id
	return nil
}

func (repository *personalTokenTestRepository) ListPersonalTokens(context.Context, string) ([]PersonalToken, error) {
	items := []PersonalToken{}
	for _, item := range repository.tokens {
		items = append(items, item)
	}
	return items, nil
}

func (repository *personalTokenTestRepository) RevokePersonalToken(_ context.Context, _, id string) error {
	item, ok := repository.tokens[id]
	if !ok {
		return ErrPersonalTokenMissing
	}
	delete(repository.tokens, id)
	for hash, tokenID := range repository.hashes {
		if tokenID == item.ID {
			delete(repository.hashes, hash)
		}
	}
	return nil
}

func (repository *personalTokenTestRepository) FindUserByPersonalTokenHash(_ context.Context, hash string, now time.Time) (domain.User, error) {
	id, ok := repository.hashes[hash]
	if !ok {
		return domain.User{}, ErrInvalidCredentials
	}
	item := repository.tokens[id]
	if item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
		return domain.User{}, ErrInvalidCredentials
	}
	usedAt := now
	item.LastUsedAt = &usedAt
	repository.tokens[id] = item
	return repository.user, nil
}

func TestPersonalTokensBDD(t *testing.T) {
	t.Run("Given a named personal token, When it is created, Then only its hash is stored and the secret is returned once", func(t *testing.T) {
		repository := &personalTokenTestRepository{user: domain.User{ID: "user-1", Role: domain.UserRole}}
		service := NewService(repository)
		expiry := time.Now().UTC().Add(24 * time.Hour)

		issued, err := service.CreatePersonalToken(context.Background(), repository.user.ID, "   home server   ", &expiry)
		if err != nil {
			t.Fatalf("create personal token: %v", err)
		}
		if !strings.HasPrefix(issued.Token, "visto_pat_") || issued.PersonalToken.Name != "home server" {
			t.Fatalf("issued token = %+v", issued)
		}
		if _, storesPlaintext := repository.hashes[issued.Token]; storesPlaintext {
			t.Fatal("repository must not store the plaintext token")
		}
		if _, ok := repository.hashes[hashToken(issued.Token)]; !ok {
			t.Fatal("repository must store the one-way token hash")
		}

		user, err := service.AuthenticatePersonalToken(context.Background(), issued.Token)
		if err != nil || user.ID != repository.user.ID {
			t.Fatalf("authenticate token: user=%+v error=%v", user, err)
		}
		listed, err := service.PersonalTokens(context.Background(), repository.user.ID)
		if err != nil || len(listed) != 1 || listed[0].LastUsedAt == nil {
			t.Fatalf("list token use: tokens=%+v error=%v", listed, err)
		}
		if err := service.RevokePersonalToken(context.Background(), repository.user.ID, issued.ID); err != nil {
			t.Fatalf("revoke token: %v", err)
		}
		if _, err := service.AuthenticatePersonalToken(context.Background(), issued.Token); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("authentication after revocation error=%v, want invalid credentials", err)
		}
	})

	t.Run("Given a token name that is blank, When creating a token, Then it is rejected", func(t *testing.T) {
		service := NewService(&personalTokenTestRepository{user: domain.User{ID: "user-1"}})
		if _, err := service.CreatePersonalToken(context.Background(), "user-1", "  ", nil); err == nil {
			t.Fatal("expected an empty token name to be rejected")
		}
	})

	t.Run("Given an expiry in the past, When creating a token, Then it is rejected", func(t *testing.T) {
		service := NewService(&personalTokenTestRepository{user: domain.User{ID: "user-1"}})
		now := time.Date(2026, time.September, 24, 12, 0, 0, 0, time.UTC)
		service.now = func() time.Time { return now }
		expiry := now.Add(-time.Second)
		if _, err := service.CreatePersonalToken(context.Background(), "user-1", "expired", &expiry); !errors.Is(err, ErrInvalidPersonalToken) {
			t.Fatalf("create error=%v, want invalid token details", err)
		}
	})
}
