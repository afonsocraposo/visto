package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

type emptyShowProvider struct{ publicTrendingProvider }

type emptySearchProvider struct{ publicTrendingProvider }

func (emptySearchProvider) Search(context.Context, string, string) ([]domain.MediaSearchResult, error) {
	return nil, nil
}

func (emptySearchProvider) Trending(context.Context, string, string) ([]domain.MediaSearchResult, error) {
	return []domain.MediaSearchResult{}, nil
}

func (emptySearchProvider) Related(context.Context, domain.MediaType, int64) ([]domain.MediaSearchResult, error) {
	return []domain.MediaSearchResult{}, nil
}

func (emptySearchProvider) Person(context.Context, int64) (domain.PersonMetadata, error) {
	return domain.PersonMetadata{TMDBID: 42, Name: "Example", Credits: []domain.PersonCredit{}}, nil
}

func (emptyShowProvider) ShowSummary(context.Context, int64) (domain.TVShowMetadata, error) {
	return domain.TVShowMetadata{TMDBID: 42, Name: "Example"}, nil
}

func (emptyShowProvider) Season(context.Context, int64, int) (domain.TVSeasonMetadata, error) {
	return domain.TVSeasonMetadata{}, nil
}

func TestTemporaryShowDetails_GivenNoCollections_ReturnsArrays(t *testing.T) {
	repository := &accountHTTPRepository{actor: domain.User{ID: "user-1", Role: domain.UserRole}}
	handler := httpserver.New(auth.NewService(repository), emptyShowProvider{}, "", nil, nil, nil, nil, nil, nil).Handler()
	request := httptest.NewRequest(http.MethodGet, "http://visto.local/api/v1/discover/shows/42", nil)
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var data struct {
		Seasons []json.RawMessage `json:"seasons"`
		Cast    []json.RawMessage `json:"cast"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Seasons == nil || data.Cast == nil {
		t.Fatalf("collections must be arrays: %s", response.Body.String())
	}
}

func TestSearch_GivenNoMatches_ReturnsEmptyArray(t *testing.T) {
	repository := &accountHTTPRepository{actor: domain.User{ID: "user-1", Role: domain.UserRole}}
	handler := httpserver.New(auth.NewService(repository), emptySearchProvider{}, "", nil, nil, nil, nil, nil, nil).Handler()
	request := httptest.NewRequest(http.MethodGet, "http://visto.local/api/v1/search?q=sopramos", nil)
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if got := strings.TrimSpace(response.Body.String()); got != "[]" {
		t.Fatalf("body=%s, want []", got)
	}
	var results []domain.MediaSearchResult
	if err := json.Unmarshal(response.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if results == nil || len(results) != 0 {
		t.Fatalf("decoded results=%v, want a non-nil empty slice", results)
	}
}

func TestDiscover_GivenEmptyLists_ReturnsArrays(t *testing.T) {
	repository := &accountHTTPRepository{actor: domain.User{ID: "user-1", Role: domain.UserRole}}
	handler := httpserver.New(auth.NewService(repository), emptySearchProvider{}, "", nil, nil, nil, nil, nil, nil).Handler()
	for _, test := range []struct {
		path string
		key  string
	}{
		{path: "/api/v1/discover/tv/42/related"},
		{path: "/api/v1/trending", key: "tv"},
		{path: "/api/v1/trending", key: "movies"},
		{path: "/api/v1/people/42", key: "credits"},
	} {
		t.Run(test.path+test.key, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var body json.RawMessage = response.Body.Bytes()
			if test.key != "" {
				var object map[string]json.RawMessage
				if err := json.Unmarshal(body, &object); err != nil {
					t.Fatal(err)
				}
				body = object[test.key]
			}
			if strings.TrimSpace(string(body)) != "[]" {
				t.Fatalf("%s=%s, want []", test.key, body)
			}
		})
	}
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
