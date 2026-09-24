package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestPublicTrending_GivenNoSession_WhenLoginRequestsArtwork_ThenItReturnsOnlyTrendingMetadata(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/trending?window=week", nil)
	response := httptest.NewRecorder()
	httpserver.New(nil, publicTrendingProvider{}, "", nil, nil, nil, nil, nil, nil).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"backdrop_path":"/example.jpg"`) {
		t.Fatalf("response has no trending backdrop: %s", response.Body.String())
	}
}

func TestRelatedMedia_GivenAuthenticatedUser_WhenARecommendationIsRequested_ThenItReturnsTMDBMediaCards(t *testing.T) {
	repository := &accountHTTPRepository{actor: domain.User{ID: "user-1", Role: domain.UserRole}}
	handler := httpserver.New(auth.NewService(repository), publicTrendingProvider{}, "", nil, nil, nil, nil, nil, nil).Handler()
	request := httptest.NewRequest(http.MethodGet, "http://visto.local/api/v1/discover/tv/42/related", nil)
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"tmdb_id":43`) || !strings.Contains(response.Body.String(), `"poster_path":"/related.jpg"`) {
		t.Fatalf("response does not contain the related media card data: %s", response.Body.String())
	}
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

func TestCreateUser_GivenAdministratorSession_WhenCreatingAnAccount_ThenItCreatesARegularUser(t *testing.T) {
	repository := &accountHTTPRepository{actor: domain.User{ID: "admin-1", Role: domain.AdminRole}}
	handler := httpserver.New(auth.NewService(repository), nil, "", nil, nil, nil, nil, nil, nil).Handler()
	request := httptest.NewRequest(http.MethodPost, "http://visto.local/api/v1/users", strings.NewReader(`{"username":"family","display_name":"Family Member","password":"correct-horse-battery-staple"}`))
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if repository.createdUser.Role != domain.UserRole || repository.createdUser.Username != "family" {
		t.Fatalf("created account = %+v, want regular user family", repository.createdUser)
	}
	if repository.passwordHash == "" || strings.Contains(repository.passwordHash, "correct-horse-battery-staple") {
		t.Fatal("new account password was not stored as a hash")
	}
}

func TestCreateUser_GivenRegularUserSession_WhenCreatingAnAccount_ThenItIsForbidden(t *testing.T) {
	repository := &accountHTTPRepository{actor: domain.User{ID: "user-1", Role: domain.UserRole}}
	handler := httpserver.New(auth.NewService(repository), nil, "", nil, nil, nil, nil, nil, nil).Handler()
	request := httptest.NewRequest(http.MethodPost, "http://visto.local/api/v1/users", strings.NewReader(`{"username":"family","display_name":"Family Member","password":"correct-horse-battery-staple"}`))
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if repository.createdUser.ID != "" {
		t.Fatalf("regular user created an account: %+v", repository.createdUser)
	}
}

func TestPersonalAPIToken_GivenAuthenticatedUser_WhenCreatedUsedListedAndRevoked_ThenSecretIsOneTimeAndBearerAccessEnds(t *testing.T) {
	repository := &personalTokenHTTPRepository{accountHTTPRepository: &accountHTTPRepository{actor: domain.User{ID: "user-1", Username: "family", Role: domain.UserRole}}}
	service := auth.NewService(repository)
	handler := httpserver.New(service, nil, "", nil, nil, nil, nil, nil, nil).Handler()
	createRequest := httptest.NewRequest(http.MethodPost, "http://visto.local/api/v1/tokens", strings.NewReader(`{"name":"home dashboard"}`))
	createRequest.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", createResponse.Code, createResponse.Body.String())
	}
	var issued auth.IssuedPersonalToken
	if err := json.Unmarshal(createResponse.Body.Bytes(), &issued); err != nil {
		t.Fatalf("decode issued token: %v", err)
	}
	if issued.Token == "" || !strings.HasPrefix(issued.Token, "visto_pat_") {
		t.Fatalf("issued token response = %+v", issued)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "http://visto.local/api/v1/tokens", nil)
	listRequest.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || strings.Contains(listResponse.Body.String(), issued.Token) {
		t.Fatalf("list response leaked token or failed: status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}

	useRequest := httptest.NewRequest(http.MethodGet, "http://visto.local/api/v1/me", nil)
	useRequest.Header.Set("Authorization", "Bearer "+issued.Token)
	useResponse := httptest.NewRecorder()
	handler.ServeHTTP(useResponse, useRequest)
	if useResponse.Code != http.StatusOK || !strings.Contains(useResponse.Body.String(), `"id":"user-1"`) {
		t.Fatalf("bearer auth status=%d body=%s", useResponse.Code, useResponse.Body.String())
	}

	revokeRequest := httptest.NewRequest(http.MethodDelete, "http://visto.local/api/v1/tokens/"+issued.ID, nil)
	revokeRequest.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d body=%s", revokeResponse.Code, revokeResponse.Body.String())
	}
	useAfterRevoke := httptest.NewRecorder()
	handler.ServeHTTP(useAfterRevoke, useRequest)
	if useAfterRevoke.Code != http.StatusUnauthorized {
		t.Fatalf("bearer auth after revocation status=%d, want 401", useAfterRevoke.Code)
	}
}

func TestHealth_GivenRunningServer_WhenHealthIsRequested_ThenItReportsOK(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	httpserver.New(auth.NewService(nil), nil, "", nil, nil, nil, nil, nil, nil).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content type = %q, want application/json", contentType)
	}
}

func TestCurrentUser_GivenNoSession_WhenRequested_ThenItRejectsTheRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	response := httptest.NewRecorder()

	httpserver.New(nil, nil, "", nil, nil, nil, nil, nil, nil).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestLogout_GivenNoSession_WhenRequested_ThenItStillClearsTheSessionCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	response := httptest.NewRecorder()
	httpserver.New(auth.NewService(nil), nil, "", nil, nil, nil, nil, nil, nil).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	cookie := response.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != "visto_session" || cookie[0].MaxAge >= 0 {
		t.Fatalf("logout cookie = %#v, want an expired visto_session cookie", cookie)
	}
}

func TestShowEpisodes_GivenNoSession_WhenRequested_ThenItRejectsTheRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/shows/tv%3A42/episodes", nil)
	response := httptest.NewRecorder()

	httpserver.New(nil, nil, "", nil, nil, nil, nil, nil, nil).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

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

func TestProtectedRoutes_GivenNoSession_WhenEveryUserScopedRouteIsCalled_ThenEachRejectsTheRequest(t *testing.T) {
	handler := httpserver.New(nil, nil, "", nil, nil, nil, nil, nil, nil).Handler()
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/users"},
		{http.MethodGet, "/api/v1/me"},
		{http.MethodGet, "/api/v1/tokens"},
		{http.MethodPost, "/api/v1/tokens"},
		{http.MethodDelete, "/api/v1/tokens/token-1"},
		{http.MethodGet, "/api/v1/profile/activity-settings"},
		{http.MethodPatch, "/api/v1/profile/activity-settings"},
		{http.MethodGet, "/api/v1/feed"},
		{http.MethodGet, "/api/v1/export/json"},
		{http.MethodGet, "/api/v1/export/csv"},
		{http.MethodGet, "/api/v1/search?q=example"},
		{http.MethodGet, "/api/v1/trending"},
		{http.MethodGet, "/api/v1/discover/movies/10"},
		{http.MethodGet, "/api/v1/discover/movie/10/related"},
		{http.MethodGet, "/api/v1/people/123"},
		{http.MethodGet, "/api/v1/discover/shows/42"},
		{http.MethodGet, "/api/v1/discover/shows/42/seasons/1"},
		{http.MethodGet, "/api/v1/discover/shows/42/seasons/1/episodes/1"},
		{http.MethodGet, "/api/v1/movies/10"},
		{http.MethodGet, "/api/v1/shows/42"},
		{http.MethodGet, "/api/v1/library"},
		{http.MethodPost, "/api/v1/library"},
		{http.MethodPatch, "/api/v1/library/tv%3A42"},
		{http.MethodPost, "/api/v1/plays"},
		{http.MethodGet, "/api/v1/plays"},
		{http.MethodPost, "/api/v1/plays/bulk"},
		{http.MethodDelete, "/api/v1/plays/bulk"},
		{http.MethodDelete, "/api/v1/plays/media/movie%3A10"},
		{http.MethodPatch, "/api/v1/plays/play-1"},
		{http.MethodDelete, "/api/v1/plays/play-1"},
		{http.MethodGet, "/api/v1/episodes/tv%3A42%3Aepisode%3A1/rating"},
		{http.MethodPut, "/api/v1/episodes/tv%3A42%3Aepisode%3A1/rating"},
		{http.MethodGet, "/api/v1/continue-watching"},
		{http.MethodGet, "/api/v1/calendar"},
		{http.MethodGet, "/api/v1/shows/tv%3A42/progress"},
		{http.MethodGet, "/api/v1/shows/tv%3A42/seasons"},
		{http.MethodGet, "/api/v1/shows/tv%3A42/episodes"},
		{http.MethodGet, "/api/v1/seasons/tv%3A42%3Aseason%3A1/episodes"},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
		})
	}
}
