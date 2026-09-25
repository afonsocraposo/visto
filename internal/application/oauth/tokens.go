package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func (service *Service) ExchangeCode(ctx context.Context, code, clientID, redirectURI, verifier, resource string) (IssuedTokens, error) {
	if code == "" || clientID == "" || !validVerifier(verifier) || resource == "" {
		return IssuedTokens{}, ErrInvalidGrant
	}
	access, err := randomToken()
	if err != nil {
		return IssuedTokens{}, err
	}
	refresh, err := randomToken()
	if err != nil {
		return IssuedTokens{}, err
	}
	now := service.now().UTC()
	identity, err := service.repository.ExchangeAuthorizationCode(ctx, hashToken(code), clientID, redirectURI, pkceChallenge(verifier), resource, hashToken(access), hashToken(refresh), now, now.Add(AccessTokenTTL), now.Add(RefreshTokenTTL))
	if err != nil {
		return IssuedTokens{}, ErrInvalidGrant
	}
	return tokenResponse(access, refresh, identity.Scopes), nil
}

func (service *Service) Refresh(ctx context.Context, refreshToken, clientID, resource string) (IssuedTokens, error) {
	if refreshToken == "" || clientID == "" || resource == "" {
		return IssuedTokens{}, ErrInvalidGrant
	}
	access, err := randomToken()
	if err != nil {
		return IssuedTokens{}, err
	}
	refresh, err := randomToken()
	if err != nil {
		return IssuedTokens{}, err
	}
	now := service.now().UTC()
	identity, err := service.repository.RotateOAuthRefreshToken(ctx, hashToken(refreshToken), clientID, resource, hashToken(access), hashToken(refresh), now, now.Add(AccessTokenTTL), now.Add(RefreshTokenTTL))
	if err != nil {
		return IssuedTokens{}, ErrInvalidGrant
	}
	return tokenResponse(access, refresh, identity.Scopes), nil
}

func (service *Service) Authenticate(ctx context.Context, token, resource string) (Identity, error) {
	if token == "" || resource == "" {
		return Identity{}, ErrInvalidToken
	}
	identity, err := service.repository.FindOAuthAccessToken(ctx, hashToken(token), resource, service.now().UTC())
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	return identity, nil
}

func (service *Service) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return service.repository.RevokeOAuthToken(ctx, hashToken(token), service.now().UTC())
}

func (service *Service) Connections(ctx context.Context, userID string) ([]Connection, error) {
	if userID == "" {
		return nil, ErrInvalidRequest
	}
	return service.repository.ListOAuthConnections(ctx, userID, service.now().UTC())
}

func (service *Service) RevokeConnection(ctx context.Context, userID, clientID string) error {
	if userID == "" || clientID == "" {
		return ErrInvalidRequest
	}
	return service.repository.RevokeOAuthConnection(ctx, userID, clientID, service.now().UTC())
}

func (service *Service) RevokeAllConnections(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalidRequest
	}
	return service.repository.RevokeAllOAuthConnections(ctx, userID, service.now().UTC())
}

// Cleanup deletes expired authorization codes immediately and removes expired
// or revoked tokens after the audit retention period. It processes one bounded
// batch, so callers can safely run it repeatedly.
func (service *Service) Cleanup(ctx context.Context) (int, error) {
	now := service.now().UTC()
	return service.repository.CleanupOAuthRecords(ctx, now, now.Add(-TokenAuditRetention), CleanupBatchSize)
}

func (service *Service) RunCleanup(ctx context.Context, interval time.Duration, report func(error)) {
	if interval <= 0 {
		return
	}
	run := func() {
		if _, err := service.Cleanup(ctx); err != nil && report != nil {
			report(err)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func tokenResponse(access, refresh string, scopes []string) IssuedTokens {
	tokens := IssuedTokens{AccessToken: access, TokenType: "Bearer", ExpiresIn: int(AccessTokenTTL.Seconds()), Scope: strings.Join(scopes, " ")}
	if contains(scopes, OfflineScope) {
		tokens.RefreshToken = refresh
	}
	return tokens
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate OAuth token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
