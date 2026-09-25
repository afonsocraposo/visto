package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

type PersonalToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type IssuedPersonalToken struct {
	PersonalToken
	Token string `json:"token"`
}

type personalTokenRepository interface {
	CreatePersonalToken(context.Context, string, string, string, string, time.Time, *time.Time) error
	ListPersonalTokens(context.Context, string) ([]PersonalToken, error)
	RevokePersonalToken(context.Context, string, string) error
	FindUserByPersonalTokenHash(context.Context, string, time.Time) (domain.User, error)
}

func (service *Service) AuthenticatePersonalToken(ctx context.Context, token string) (domain.User, error) {
	if !strings.HasPrefix(token, "visto_pat_") {
		return domain.User{}, ErrInvalidCredentials
	}
	repository, ok := service.repository.(personalTokenRepository)
	if !ok {
		return domain.User{}, ErrPersonalTokensUnavailable
	}
	return repository.FindUserByPersonalTokenHash(ctx, hashToken(token), service.now().UTC())
}

func (service *Service) CreatePersonalToken(ctx context.Context, userID, name string, expiresAt *time.Time) (IssuedPersonalToken, error) {
	if userID == "" {
		return IssuedPersonalToken{}, fmt.Errorf("user is required")
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 {
		return IssuedPersonalToken{}, fmt.Errorf("%w: name must be 1–80 characters", ErrInvalidPersonalToken)
	}
	now := service.now().UTC()
	if expiresAt != nil {
		expiresAt = ptrTime(expiresAt.UTC())
		if !expiresAt.After(now) {
			return IssuedPersonalToken{}, fmt.Errorf("%w: expiry must be in the future", ErrInvalidPersonalToken)
		}
	}
	repository, ok := service.repository.(personalTokenRepository)
	if !ok {
		return IssuedPersonalToken{}, ErrPersonalTokensUnavailable
	}
	rawToken, err := newToken()
	if err != nil {
		return IssuedPersonalToken{}, fmt.Errorf("generate personal API token: %w", err)
	}
	rawToken = "visto_pat_" + rawToken
	item := PersonalToken{ID: newID(), Name: name, CreatedAt: now, ExpiresAt: expiresAt}
	if err := repository.CreatePersonalToken(ctx, item.ID, userID, name, hashToken(rawToken), now, expiresAt); err != nil {
		return IssuedPersonalToken{}, err
	}
	return IssuedPersonalToken{PersonalToken: item, Token: rawToken}, nil
}

func (service *Service) PersonalTokens(ctx context.Context, userID string) ([]PersonalToken, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	repository, ok := service.repository.(personalTokenRepository)
	if !ok {
		return nil, ErrPersonalTokensUnavailable
	}
	return repository.ListPersonalTokens(ctx, userID)
}

func (service *Service) RevokePersonalToken(ctx context.Context, userID, tokenID string) error {
	if userID == "" || tokenID == "" {
		return fmt.Errorf("user and token are required")
	}
	repository, ok := service.repository.(personalTokenRepository)
	if !ok {
		return ErrPersonalTokensUnavailable
	}
	return repository.RevokePersonalToken(ctx, userID, tokenID)
}

func hashToken(token string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(token))) }

func ptrTime(value time.Time) *time.Time { return &value }

func newID() string { token, _ := newToken(); return token }

func newToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
