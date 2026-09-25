package oauth

import (
	"context"
	"errors"
	"regexp"
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
	ReadScope           = "read"
	WriteScope          = "write"
	OfflineScope        = "offline_access"
	AccessTokenTTL      = time.Hour
	RefreshTokenTTL     = 30 * 24 * time.Hour
	TokenAuditRetention = 30 * 24 * time.Hour
	CleanupBatchSize    = 100
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
	CleanupOAuthRecords(context.Context, time.Time, time.Time, int) (int, error)
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
