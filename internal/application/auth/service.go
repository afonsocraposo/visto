package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrBootstrapComplete  = errors.New("initial administrator already exists")
)

type Repository interface {
	BootstrapAdmin(context.Context, domain.User, string) error
	CreateUser(context.Context, domain.User, string) error
	FindUserByUsername(context.Context, string) (domain.User, string, error)
	CreateSession(context.Context, string, string, string, time.Time) error
	FindUserBySessionToken(context.Context, string, time.Time) (domain.User, error)
}

func (service *Service) CreateUser(ctx context.Context, username, displayName, password string) (domain.User, error) {
	username, displayName, err := validateAccount(username, displayName, password)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	user := domain.User{ID: newID(), Username: username, DisplayName: displayName, Role: domain.UserRole, CreatedAt: service.now().UTC()}
	if err := service.repository.CreateUser(ctx, user, hash); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (service *Service) Bootstrap(ctx context.Context, username, displayName, password string) (domain.User, error) {
	username, displayName, err := validateAccount(username, displayName, password)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	user := domain.User{ID: newID(), Username: username, DisplayName: displayName, Role: domain.AdminRole, CreatedAt: service.now().UTC()}
	if err := service.repository.BootstrapAdmin(ctx, user, hash); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (service *Service) Login(ctx context.Context, username, password string) (domain.User, string, time.Time, error) {
	user, hash, err := service.repository.FindUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil || !verifyPassword(hash, password) {
		return domain.User{}, "", time.Time{}, ErrInvalidCredentials
	}
	token, err := newToken()
	if err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	expiresAt := service.now().UTC().Add(30 * 24 * time.Hour)
	if err := service.repository.CreateSession(ctx, newID(), user.ID, hashToken(token), expiresAt); err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	return user, token, expiresAt, nil
}

func (service *Service) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, ErrInvalidCredentials
	}
	return service.repository.FindUserBySessionToken(ctx, hashToken(token), service.now().UTC())
}

func validateAccount(username, displayName, password string) (string, string, error) {
	username, displayName = strings.TrimSpace(username), strings.TrimSpace(displayName)
	if len(username) < 3 || len(username) > 32 || strings.ContainsAny(username, " \t\n") {
		return "", "", fmt.Errorf("username must be 3–32 characters without spaces")
	}
	if displayName == "" || len(displayName) > 80 {
		return "", "", fmt.Errorf("display name must be 1–80 characters")
	}
	if len(password) < 12 {
		return "", "", fmt.Errorf("password must be at least 12 characters")
	}
	return username, displayName, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return "$visto$argon2id$v=19$m=65536,t=3,p=4$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 7 || parts[1] != "visto" || parts[2] != "argon2id" {
		return false
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[5])
	expected, keyErr := base64.RawStdEncoding.DecodeString(parts[6])
	if saltErr != nil || keyErr != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, uint32(len(expected)))
	return subtleCompare(actual, expected)
}

func subtleCompare(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var result byte
	for i := range left {
		result |= left[i] ^ right[i]
	}
	return result == 0
}
func hashToken(token string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(token))) }
func newID() string                 { token, _ := newToken(); return token }
func newToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
