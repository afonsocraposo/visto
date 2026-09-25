package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

var (
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrBootstrapComplete         = errors.New("initial administrator already exists")
	ErrPersonalTokenMissing      = errors.New("personal API token not found")
	ErrInvalidPersonalToken      = errors.New("invalid personal API token details")
	ErrPersonalTokensUnavailable = errors.New("personal API tokens are not configured")
)

type Repository interface {
	BootstrapAdmin(context.Context, domain.User, string) error
	CreateUser(context.Context, domain.User, string) error
	FindUserByUsername(context.Context, string) (domain.User, string, error)
	CreateSession(context.Context, string, string, string, time.Time) error
	FindUserBySessionToken(context.Context, string, time.Time) (domain.User, error)
	RevokeSession(context.Context, string) error
}

type accountCounter interface {
	UserCount(context.Context) (int, error)
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

func (service *Service) BootstrapAvailable(ctx context.Context) (bool, error) {
	repository, ok := service.repository.(accountCounter)
	if !ok {
		return false, fmt.Errorf("account status is not configured")
	}
	count, err := repository.UserCount(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
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
	user, err := service.AuthenticateCredentials(ctx, username, password)
	if err != nil {
		return domain.User{}, "", time.Time{}, err
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

func (service *Service) AuthenticateCredentials(ctx context.Context, username, password string) (domain.User, error) {
	user, hash, err := service.repository.FindUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil || !verifyPassword(hash, password) {
		return domain.User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (service *Service) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, ErrInvalidCredentials
	}
	return service.repository.FindUserBySessionToken(ctx, hashToken(token), service.now().UTC())
}

func (service *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return service.repository.RevokeSession(ctx, hashToken(token))
}
