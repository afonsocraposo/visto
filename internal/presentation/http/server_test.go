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
