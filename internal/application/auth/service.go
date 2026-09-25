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
	ErrBootstrapIncomplete       = errors.New("the administrator must finish setup before signup is available")
	ErrSignupsDisabled           = errors.New("public signups are disabled")
	ErrUserNotFound              = errors.New("user not found")
	ErrLastAdministrator         = errors.New("cannot remove the last administrator")
	ErrPersonalTokenMissing      = errors.New("personal API token not found")
	ErrInvalidPersonalToken      = errors.New("invalid personal API token details")
	ErrPersonalTokensUnavailable = errors.New("personal API tokens are not configured")
)

type Repository interface {
	BootstrapAdmin(context.Context, domain.User, string) error
	CreateUser(context.Context, domain.User, string) error
	FindUserByEmail(context.Context, string) (domain.User, string, error)
	CreateSession(context.Context, string, string, string, time.Time) error
	FindUserBySessionToken(context.Context, string, time.Time) (domain.User, error)
	RevokeSession(context.Context, string) error
}

type googleRepository interface {
	FindUserByGoogleSubject(context.Context, string) (domain.User, error)
	LinkGoogleSubject(context.Context, string, string) error
	CreateGoogleUser(context.Context, domain.User, string) error
}

type accountCounter interface {
	AdminCount(context.Context) (int, error)
}

type signupRepository interface {
	CreateSignupUser(context.Context, domain.User, string) error
}

type adminUsersRepository interface {
	ListUsers(context.Context) ([]domain.User, error)
	UpdateUser(context.Context, string, string, string, string) error
	DeleteUser(context.Context, string) error
}

type Config struct {
	AllowSignups  bool
	GoogleEnabled bool
}

func (service *Service) CreateUser(ctx context.Context, email, displayName, password string) (domain.User, error) {
	email, displayName, err := validateAccount(email, displayName, password)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	user := domain.User{ID: newID(), Email: email, DisplayName: displayName, Role: domain.UserRole, CreatedAt: service.now().UTC()}
	if err := service.repository.CreateUser(ctx, user, hash); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

type Service struct {
	repository    Repository
	now           func() time.Time
	allowSignups  bool
	googleEnabled bool
}

func NewService(repository Repository, configs ...Config) *Service {
	service := &Service{repository: repository, now: time.Now, allowSignups: true}
	if len(configs) > 0 {
		service.allowSignups = configs[0].AllowSignups
		service.googleEnabled = configs[0].GoogleEnabled
	}
	return service
}

func (service *Service) BootstrapAvailable(ctx context.Context) (bool, error) {
	repository, ok := service.repository.(accountCounter)
	if !ok {
		return false, fmt.Errorf("account status is not configured")
	}
	count, err := repository.AdminCount(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (service *Service) SignupEnabled() bool { return service.allowSignups }

func (service *Service) SignUp(ctx context.Context, email, name, password string) (domain.User, string, time.Time, error) {
	if !service.allowSignups {
		return domain.User{}, "", time.Time{}, ErrSignupsDisabled
	}
	repository, ok := service.repository.(signupRepository)
	if !ok {
		return domain.User{}, "", time.Time{}, fmt.Errorf("signup is not configured")
	}
	email, displayName, err := validateAccount(email, name, password)
	if err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	user := domain.User{ID: newID(), Email: email, DisplayName: displayName, Role: domain.UserRole, CreatedAt: service.now().UTC()}
	if err := repository.CreateSignupUser(ctx, user, hash); err != nil {
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

func (service *Service) Users(ctx context.Context) ([]domain.User, error) {
	repository, ok := service.repository.(adminUsersRepository)
	if !ok {
		return nil, fmt.Errorf("user management is not configured")
	}
	return repository.ListUsers(ctx)
}

func (service *Service) UpdateUser(ctx context.Context, userID, displayName, password, keepSessionToken string) error {
	repository, ok := service.repository.(adminUsersRepository)
	if !ok {
		return fmt.Errorf("user management is not configured")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len(displayName) > 80 {
		return fmt.Errorf("display name must be 1–80 characters")
	}
	passwordHash := ""
	if password != "" {
		if len(password) < 12 {
			return fmt.Errorf("password must be at least 12 characters")
		}
		var err error
		passwordHash, err = hashPassword(password)
		if err != nil {
			return err
		}
	}
	keepSessionHash := ""
	if keepSessionToken != "" {
		keepSessionHash = hashToken(keepSessionToken)
	}
	return repository.UpdateUser(ctx, userID, displayName, passwordHash, keepSessionHash)
}

func (service *Service) DeleteUser(ctx context.Context, userID string) error {
	repository, ok := service.repository.(adminUsersRepository)
	if !ok {
		return fmt.Errorf("user management is not configured")
	}
	return repository.DeleteUser(ctx, userID)
}

func (service *Service) Bootstrap(ctx context.Context, email, displayName, password string) (domain.User, error) {
	email, displayName, err := validateAccount(email, displayName, password)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	user := domain.User{ID: newID(), Email: email, DisplayName: displayName, Role: domain.AdminRole, CreatedAt: service.now().UTC()}
	if err := service.repository.BootstrapAdmin(ctx, user, hash); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (service *Service) Login(ctx context.Context, email, password string) (domain.User, string, time.Time, error) {
	user, err := service.AuthenticateCredentials(ctx, email, password)
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

func (service *Service) AuthenticateCredentials(ctx context.Context, email, password string) (domain.User, error) {
	user, hash, err := service.repository.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || !verifyPassword(hash, password) {
		return domain.User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (service *Service) GoogleEnabled() bool { return service.googleEnabled }

func (service *Service) LoginWithGoogle(ctx context.Context, subject, email, name string) (domain.User, string, time.Time, error) {
	if !service.googleEnabled || subject == "" {
		return domain.User{}, "", time.Time{}, ErrInvalidCredentials
	}
	repository, ok := service.repository.(googleRepository)
	if !ok {
		return domain.User{}, "", time.Time{}, fmt.Errorf("Google sign-in is not configured")
	}
	user, err := repository.FindUserByGoogleSubject(ctx, subject)
	if errors.Is(err, ErrInvalidCredentials) {
		email, name, err = validateIdentity(email, name)
		if err != nil {
			return domain.User{}, "", time.Time{}, err
		}
		localUser, _, lookupErr := service.repository.FindUserByEmail(ctx, email)
		if lookupErr == nil {
			user = localUser
			if err := repository.LinkGoogleSubject(ctx, user.ID, subject); err != nil {
				return domain.User{}, "", time.Time{}, err
			}
		} else if errors.Is(lookupErr, ErrInvalidCredentials) {
			if !service.allowSignups {
				return domain.User{}, "", time.Time{}, ErrSignupsDisabled
			}
			user = domain.User{ID: newID(), Email: email, DisplayName: name, Role: domain.UserRole, CreatedAt: service.now().UTC()}
			if err := repository.CreateGoogleUser(ctx, user, subject); err != nil {
				return domain.User{}, "", time.Time{}, err
			}
		} else {
			return domain.User{}, "", time.Time{}, lookupErr
		}
	} else if err != nil {
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
