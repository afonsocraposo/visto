package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

type accountHTTPRepository struct {
	actor        domain.User
	createdUser  domain.User
	passwordHash string
}

type personalTokenHTTPRepository struct {
	*accountHTTPRepository
	tokenHash string
	tokens    []auth.PersonalToken
}

func (repository *personalTokenHTTPRepository) CreatePersonalToken(_ context.Context, id, _ string, name, tokenHash string, createdAt time.Time, expiresAt *time.Time) error {
	repository.tokenHash = tokenHash
	repository.tokens = append(repository.tokens, auth.PersonalToken{ID: id, Name: name, CreatedAt: createdAt, ExpiresAt: expiresAt})
	return nil
}

func (repository *personalTokenHTTPRepository) ListPersonalTokens(context.Context, string) ([]auth.PersonalToken, error) {
	return repository.tokens, nil
}

func (repository *personalTokenHTTPRepository) RevokePersonalToken(_ context.Context, _, tokenID string) error {
	for index, token := range repository.tokens {
		if token.ID == tokenID {
			repository.tokens = append(repository.tokens[:index], repository.tokens[index+1:]...)
			repository.tokenHash = ""
			return nil
		}
	}
	return auth.ErrPersonalTokenMissing
}

func (repository *personalTokenHTTPRepository) FindUserByPersonalTokenHash(_ context.Context, tokenHash string, _ time.Time) (domain.User, error) {
	if tokenHash != repository.tokenHash || tokenHash == "" {
		return domain.User{}, auth.ErrInvalidCredentials
	}
	return repository.actor, nil
}

type publicTrendingProvider struct{}

func (publicTrendingProvider) Search(context.Context, string, string) ([]domain.MediaSearchResult, error) {
	return nil, nil
}

func (publicTrendingProvider) Trending(_ context.Context, mediaType, _ string) ([]domain.MediaSearchResult, error) {
	return []domain.MediaSearchResult{{TMDBID: 42, Type: domain.MediaType(mediaType), Title: "Example", BackdropPath: "/example.jpg"}}, nil
}

func (publicTrendingProvider) Related(_ context.Context, mediaType domain.MediaType, tmdbID int64) ([]domain.MediaSearchResult, error) {
	return []domain.MediaSearchResult{{TMDBID: tmdbID + 1, Type: mediaType, Title: "Related title", PosterPath: "/related.jpg"}}, nil
}

func (*accountHTTPRepository) BootstrapAdmin(context.Context, domain.User, string) error { return nil }

func (repository *accountHTTPRepository) CreateUser(_ context.Context, user domain.User, passwordHash string) error {
	repository.createdUser = user
	repository.passwordHash = passwordHash
	return nil
}

func (*accountHTTPRepository) FindUserByUsername(context.Context, string) (domain.User, string, error) {
	return domain.User{}, "", nil
}

func (*accountHTTPRepository) CreateSession(context.Context, string, string, string, time.Time) error {
	return nil
}

func (repository *accountHTTPRepository) FindUserBySessionToken(context.Context, string, time.Time) (domain.User, error) {
	return repository.actor, nil
}

func (*accountHTTPRepository) RevokeSession(context.Context, string) error { return nil }

func TestShowCatalog_GivenNoSession_WhenProgressSeasonsOrEpisodesAreRequested_ThenItRejectsEachRequest(t *testing.T) {
	handler := httpserver.New(nil, nil, "", nil, nil, nil, nil, nil, nil).Handler()
	for _, path := range []string{
		"/api/v1/movies/42",
		"/api/v1/shows/42",
		"/api/v1/shows/tv%3A42/progress",
		"/api/v1/shows/tv%3A42/seasons",
		"/api/v1/seasons/tv%3A42%3Aseason%3A15/episodes",
	} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
		})
	}
}
