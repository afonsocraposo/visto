package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

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

func TestProtectedRoutes_GivenNoSession_WhenEveryUserScopedRouteIsCalled_ThenEachRejectsTheRequest(t *testing.T) {
	handler := httpserver.New(nil, nil, "", nil, nil, nil, nil, nil, nil).Handler()
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/users"},
		{http.MethodPost, "/api/v1/users"},
		{http.MethodPatch, "/api/v1/users/user-1"},
		{http.MethodDelete, "/api/v1/users/user-1"},
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
		{http.MethodDelete, "/api/v1/library/tv%3A42"},
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
