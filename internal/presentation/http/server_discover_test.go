package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

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
