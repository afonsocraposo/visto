package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidRequest = errors.New("invalid OAuth request")
	ErrInvalidClient  = errors.New("invalid OAuth client")
	ErrInvalidGrant   = errors.New("invalid OAuth grant")
	ErrInvalidToken   = errors.New("invalid OAuth token")
)

const (
	ReadScope       = "read"
	WriteScope      = "write"
	OfflineScope    = "offline_access"
	AccessTokenTTL  = time.Hour
	RefreshTokenTTL = 30 * 24 * time.Hour
)

var verifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

type Client struct {
	ID           string    `json:"client_id"`
	Name         string    `json:"client_name"`
	RedirectURIs []string  `json:"redirect_uris"`
	CreatedAt    time.Time `json:"created_at"`
}

type AuthorizationCode struct {
	Hash        string
	ClientID    string
	UserID      string
	RedirectURI string
	Challenge   string
	Scopes      []string
	Resource    string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

type Identity struct {
	UserID   string
	ClientID string
	Scopes   []string
	Resource string
	Expires  time.Time
}

type IssuedTokens struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

// Connection is an OAuth client that currently has access to a user's Visto
// account. Tokens are never included in this response.
type Connection struct {
	ClientID    string     `json:"client_id"`
	ClientName  string     `json:"client_name"`
	Scopes      []string   `json:"scopes"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

type Repository interface {
	RegisterOAuthClient(context.Context, Client) error
	FindOAuthClient(context.Context, string) (Client, error)
	CreateAuthorizationCode(context.Context, AuthorizationCode) error
	ExchangeAuthorizationCode(context.Context, string, string, string, string, string, string, string, time.Time, time.Time, time.Time) (Identity, error)
	RotateOAuthRefreshToken(context.Context, string, string, string, string, string, time.Time, time.Time, time.Time) (Identity, error)
	FindOAuthAccessToken(context.Context, string, string, time.Time) (Identity, error)
	RevokeOAuthToken(context.Context, string, time.Time) error
	ListOAuthConnections(context.Context, string, time.Time) ([]Connection, error)
	RevokeOAuthConnection(context.Context, string, string, time.Time) error
	RevokeAllOAuthConnections(context.Context, string, time.Time) error
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (service *Service) RegisterClient(ctx context.Context, name string, redirectURIs []string) (Client, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 || len(redirectURIs) == 0 || len(redirectURIs) > 20 {
		return Client{}, ErrInvalidClient
	}
	seen := map[string]bool{}
	for _, redirectURI := range redirectURIs {
		if !validRedirectURI(redirectURI) || seen[redirectURI] {
			return Client{}, ErrInvalidClient
		}
		seen[redirectURI] = true
	}
	id, err := randomToken()
	if err != nil {
		return Client{}, err
	}
	client := Client{ID: "visto_client_" + id, Name: name, RedirectURIs: redirectURIs, CreatedAt: service.now().UTC()}
	if err := service.repository.RegisterOAuthClient(ctx, client); err != nil {
		return Client{}, err
	}
	return client, nil
}

func (service *Service) Client(ctx context.Context, clientID string) (Client, error) {
	client, err := service.repository.FindOAuthClient(ctx, clientID)
	if err != nil {
		return Client{}, ErrInvalidClient
	}
	return client, nil
}

type AuthorizeRequest struct {
	ClientID        string
	RedirectURI     string
	State           string
	Scopes          []string
	Resource        string
	CodeChallenge   string
	ChallengeMethod string
}

func (service *Service) CreateAuthorizationCode(ctx context.Context, userID string, request AuthorizeRequest, expectedResource string) (string, error) {
	client, err := service.Client(ctx, request.ClientID)
	if err != nil || !contains(client.RedirectURIs, request.RedirectURI) || userID == "" || request.Resource != expectedResource || request.ChallengeMethod != "S256" || !validChallenge(request.CodeChallenge) {
		return "", ErrInvalidRequest
	}
	scopes, err := normalizeScopes(request.Scopes)
	if err != nil {
		return "", err
	}
	code, err := randomToken()
	if err != nil {
		return "", err
	}
	now := service.now().UTC()
	item := AuthorizationCode{Hash: hashToken(code), ClientID: client.ID, UserID: userID, RedirectURI: request.RedirectURI, Challenge: request.CodeChallenge, Scopes: scopes, Resource: request.Resource, ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now}
	if err := service.repository.CreateAuthorizationCode(ctx, item); err != nil {
		return "", err
	}
	return code, nil
}

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

func tokenResponse(access, refresh string, scopes []string) IssuedTokens {
	tokens := IssuedTokens{AccessToken: access, TokenType: "Bearer", ExpiresIn: int(AccessTokenTTL.Seconds()), Scope: strings.Join(scopes, " ")}
	if contains(scopes, OfflineScope) {
		tokens.RefreshToken = refresh
	}
	return tokens
}

func normalizeScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return []string{ReadScope, WriteScope, OfflineScope}, nil
	}
	seen := map[string]bool{}
	for _, scope := range scopes {
		if scope != ReadScope && scope != WriteScope && scope != OfflineScope {
			return nil, ErrInvalidRequest
		}
		seen[scope] = true
	}
	if !seen[ReadScope] && !seen[WriteScope] {
		return nil, ErrInvalidRequest
	}
	result := make([]string, 0, len(seen))
	for scope := range seen {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result, nil
}

func validRedirectURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Fragment != "" || parsed.User != nil {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func validChallenge(challenge string) bool {
	if len(challenge) < 43 || len(challenge) > 128 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	return err == nil && len(decoded) == sha256.Size
}

func validVerifier(verifier string) bool { return verifierPattern.MatchString(verifier) }

func pkceChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func verifyPKCE(challenge, verifier string) bool {
	computed := pkceChallenge(verifier)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

func contains(values []string, item string) bool {
	for _, value := range values {
		if value == item {
			return true
		}
	}
	return false
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
