package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
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

type mixedSearchProvider struct{ publicTrendingProvider }

func (mixedSearchProvider) Search(context.Context, string, string) ([]domain.MediaSearchResult, error) {
	return []domain.MediaSearchResult{{TMDBID: 1, Type: domain.TVMediaType, Title: "Show"}, {TMDBID: 2, Type: domain.MovieMediaType, Title: "Film"}}, nil
}

type typedSearchProvider struct {
	mixedSearchProvider
	requested *domain.MediaType
}

func (provider typedSearchProvider) SearchType(_ context.Context, mediaType domain.MediaType, _, _ string) ([]domain.MediaSearchResult, error) {
	*provider.requested = mediaType
	return []domain.MediaSearchResult{{TMDBID: 3, Type: mediaType, Title: "Typed"}}, nil
}

func searchTypes(t *testing.T, provider domain.MetadataProvider, query string) (int, []domain.MediaType) {
	t.Helper()
	repository := &accountHTTPRepository{actor: domain.User{ID: "user-1", Role: domain.UserRole}}
	handler := httpserver.New(auth.NewService(repository), provider, "", nil, nil, nil, nil, nil, nil).Handler()
	request := httptest.NewRequest(http.MethodGet, "http://visto.local/api/v1/search?q=example"+query, nil)
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		return response.Code, nil
	}
	var results []domain.MediaSearchResult
	if err := json.Unmarshal(response.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	types := []domain.MediaType{}
	for _, result := range results {
		types = append(types, result.Type)
	}
	return response.Code, types
}

func TestSearch_GivenMediaTypeFilter_ReturnsOnlyThatType(t *testing.T) {
	for _, test := range []struct {
		query string
		want  []domain.MediaType
	}{
		{query: "", want: []domain.MediaType{domain.TVMediaType, domain.MovieMediaType}},
		{query: "&type=all", want: []domain.MediaType{domain.TVMediaType, domain.MovieMediaType}},
		{query: "&type=tv", want: []domain.MediaType{domain.TVMediaType}},
		{query: "&type=movie", want: []domain.MediaType{domain.MovieMediaType}},
	} {
		t.Run(test.query, func(t *testing.T) {
			status, types := searchTypes(t, mixedSearchProvider{}, test.query)
			if status != http.StatusOK || !slices.Equal(types, test.want) {
				t.Fatalf("status=%d types=%v, want %v", status, types, test.want)
			}
		})
	}
}

func TestSearch_GivenTypedProvider_WhenFiltered_ThenItSearchesThatTypeAtTheSource(t *testing.T) {
	var requested domain.MediaType
	status, types := searchTypes(t, typedSearchProvider{requested: &requested}, "&type=movie")
	if status != http.StatusOK || requested != domain.MovieMediaType || !slices.Equal(types, []domain.MediaType{domain.MovieMediaType}) {
		t.Fatalf("status=%d requested=%q types=%v", status, requested, types)
	}
	requested = ""
	if status, _ := searchTypes(t, typedSearchProvider{requested: &requested}, ""); status != http.StatusOK || requested != "" {
		t.Fatalf("status=%d requested=%q, want the mixed search for All", status, requested)
	}
}

func TestSearch_GivenUnknownMediaType_ReturnsBadRequest(t *testing.T) {
	if status, _ := searchTypes(t, mixedSearchProvider{}, "&type=person"); status != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", status)
	}
}
