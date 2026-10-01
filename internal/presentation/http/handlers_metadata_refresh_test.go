package httpserver_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

type refreshMetadataProvider struct {
	show   domain.TVShowMetadata
	movie  domain.MovieMetadata
	err    error
	called int
}

func (*refreshMetadataProvider) Search(context.Context, string, string) ([]domain.MediaSearchResult, error) {
	return nil, nil
}

func (provider *refreshMetadataProvider) Show(context.Context, int64) (domain.TVShowMetadata, error) {
	provider.called++
	return provider.show, provider.err
}

func (provider *refreshMetadataProvider) Movie(context.Context, int64) (domain.MovieMetadata, error) {
	provider.called++
	return provider.movie, provider.err
}

type metadataRefreshFixture struct {
	store    *sqlite.Store
	provider *refreshMetadataProvider
	handler  http.Handler
}

func newMetadataRefreshFixture(t *testing.T) metadataRefreshFixture {
	t.Helper()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	stamp := "2026-09-24T00:00:00Z"
	for _, user := range []struct{ id, name, role, session string }{
		{"1", "Admin", "admin", "admin-session"},
		{"2", "Viewer", "user", "viewer-session"},
	} {
		if _, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at,email) VALUES(?,?,?,'hash',?,?,?,?)`, user.id, user.name, user.name, user.role, stamp, stamp, user.name+"@example.test"); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(user.session))
		if _, err := store.DB.Exec(`INSERT INTO sessions(user_id,token_hash,expires_at,created_at) VALUES(?,?,'2099-01-01T00:00:00Z',?)`, user.id, hex.EncodeToString(hash[:]), stamp); err != nil {
			t.Fatal(err)
		}
	}
	// A deliberately corrupted show: metadata is marked fresh but its overview and artwork are gone.
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,overview,poster_path,backdrop_path,metadata_updated_at,catalog_updated_at,created_at) VALUES('tv:2316','tv',2316,'The Office','','','',?,?,?)`, stamp, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:2316:season:1','tv:2316',1,'Season 1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name,tmdb_id) VALUES('tv:2316:episode:1','tv:2316','tv:2316:season:1',1,1,'Pilot',1001)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at) VALUES('item-1',2,'tv:2316','watching',4,?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES(2,'tv:2316:episode:1',?,'web',?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,overview,poster_path,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Broken','','',?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	provider := &refreshMetadataProvider{
		show: domain.TVShowMetadata{TMDBID: 2316, Name: "The Office", Overview: "A mockumentary.", PosterPath: "/office.jpg", BackdropPath: "/office-backdrop.jpg", Status: "Ended", Seasons: []domain.TVSeasonMetadata{
			{Number: 1, Name: "Season 1", Episodes: []domain.TVEpisodeMetadata{
				{TMDBID: 1001, SeasonNumber: 1, EpisodeNumber: 1, Name: "Pilot"},
				{TMDBID: 1002, SeasonNumber: 1, EpisodeNumber: 2, Name: "Diversity Day"},
			}},
		}},
		movie: domain.MovieMetadata{TMDBID: 10, Title: "Fixed", Overview: "Restored overview.", PosterPath: "/fixed.jpg"},
	}
	handler := httpserver.New(auth.NewService(store), provider, "", library.NewService(store), nil, nil, nil, nil, nil).Handler()
	return metadataRefreshFixture{store: store, provider: provider, handler: handler}
}

func (fixture metadataRefreshFixture) refresh(path, session string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, nil)
	if session != "" {
		request.AddCookie(&http.Cookie{Name: "visto_session", Value: session})
	}
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	return response
}

func TestRefreshMediaMetadata_GivenNonAdministrators_WhenRefreshing_ThenItRejectsThemBeforeCallingTMDB(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	if response := fixture.refresh("/api/v1/media/tv/2316/refresh", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d %s", response.Code, response.Body.String())
	}
	if response := fixture.refresh("/api/v1/media/tv/2316/refresh", "viewer-session"); response.Code != http.StatusForbidden {
		t.Fatalf("viewer = %d %s", response.Code, response.Body.String())
	}
	if fixture.provider.called != 0 {
		t.Fatalf("provider called %d times", fixture.provider.called)
	}
}

func TestRefreshMediaMetadata_GivenInvalidOrUnknownMedia_WhenRefreshing_ThenItReportsTheProblem(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	for path, want := range map[string]int{
		"/api/v1/media/person/2316/refresh": http.StatusBadRequest,
		"/api/v1/media/tv/abc/refresh":      http.StatusBadRequest,
		"/api/v1/media/tv/0/refresh":        http.StatusBadRequest,
		"/api/v1/media/movie/999/refresh":   http.StatusNotFound,
	} {
		if response := fixture.refresh(path, "admin-session"); response.Code != want {
			t.Fatalf("%s = %d %s, want %d", path, response.Code, response.Body.String(), want)
		}
	}
	if fixture.provider.called != 0 {
		t.Fatalf("provider called %d times", fixture.provider.called)
	}
}

func TestRefreshMediaMetadata_GivenCorruptedShow_WhenAnAdministratorRefreshes_ThenMetadataAndCatalogAreRestoredWithoutLosingUserData(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	if response := fixture.refresh("/api/v1/media/tv/2316/refresh", "admin-session"); response.Code != http.StatusNoContent {
		t.Fatalf("refresh = %d %s", response.Code, response.Body.String())
	}
	var overview, poster, backdrop, metadataUpdated, catalogUpdated string
	if err := fixture.store.DB.QueryRow(`SELECT overview,poster_path,backdrop_path,metadata_updated_at,catalog_updated_at FROM media WHERE id='tv:2316'`).Scan(&overview, &poster, &backdrop, &metadataUpdated, &catalogUpdated); err != nil {
		t.Fatal(err)
	}
	if overview != "A mockumentary." || poster != "/office.jpg" || backdrop != "/office-backdrop.jpg" {
		t.Fatalf("metadata = %q %q %q", overview, poster, backdrop)
	}
	if metadataUpdated == "2026-09-24T00:00:00Z" || catalogUpdated == "2026-09-24T00:00:00Z" {
		t.Fatalf("timestamps not refreshed: metadata=%s catalog=%s", metadataUpdated, catalogUpdated)
	}
	var episodes, plays, rating int
	var status string
	if err := fixture.store.DB.QueryRow(`SELECT COUNT(*) FROM episodes WHERE show_id='tv:2316' AND active=1`).Scan(&episodes); err != nil || episodes != 2 {
		t.Fatalf("episodes = %d: %v", episodes, err)
	}
	if err := fixture.store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE episode_id='tv:2316:episode:1'`).Scan(&plays); err != nil || plays != 1 {
		t.Fatalf("plays = %d: %v", plays, err)
	}
	if err := fixture.store.DB.QueryRow(`SELECT status,rating FROM user_media WHERE id='item-1'`).Scan(&status, &rating); err != nil || status != "watching" || rating != 4 {
		t.Fatalf("library item = %s %d: %v", status, rating, err)
	}
}

func TestRefreshMediaMetadata_GivenStaleMovie_WhenAnAdministratorRefreshes_ThenTMDBMetadataReplacesIt(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	if response := fixture.refresh("/api/v1/media/movie/10/refresh", "admin-session"); response.Code != http.StatusNoContent {
		t.Fatalf("refresh = %d %s", response.Code, response.Body.String())
	}
	var title, overview, poster string
	if err := fixture.store.DB.QueryRow(`SELECT title,overview,poster_path FROM media WHERE id='movie:10'`).Scan(&title, &overview, &poster); err != nil {
		t.Fatal(err)
	}
	if title != "Fixed" || overview != "Restored overview." || poster != "/fixed.jpg" {
		t.Fatalf("movie = %q %q %q", title, overview, poster)
	}
}

func TestRefreshMediaMetadata_GivenTMDBFailure_WhenRefreshing_ThenStoredMetadataIsUntouched(t *testing.T) {
	for name, path := range map[string]string{"tv": "/api/v1/media/tv/2316/refresh", "movie": "/api/v1/media/movie/10/refresh"} {
		t.Run(name, func(t *testing.T) {
			fixture := newMetadataRefreshFixture(t)
			fixture.provider.err = errors.New("TMDB unavailable")
			if response := fixture.refresh(path, "admin-session"); response.Code != http.StatusBadGateway {
				t.Fatalf("refresh = %d %s", response.Code, response.Body.String())
			}
			var title, updated string
			if err := fixture.store.DB.QueryRow(`SELECT title,metadata_updated_at FROM media WHERE media_type=?`, name).Scan(&title, &updated); err != nil {
				t.Fatal(err)
			}
			if updated != "2026-09-24T00:00:00Z" || (name == "movie" && title != "Broken") {
				t.Fatalf("metadata changed: %q %s", title, updated)
			}
		})
	}
}

func TestRefreshMediaMetadata_GivenInvalidTMDBResponse_WhenRefreshing_ThenStoredMetadataIsUntouched(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	fixture.provider.movie = domain.MovieMetadata{TMDBID: 10}
	if response := fixture.refresh("/api/v1/media/movie/10/refresh", "admin-session"); response.Code != http.StatusBadGateway {
		t.Fatalf("refresh = %d %s", response.Code, response.Body.String())
	}
	var title string
	if err := fixture.store.DB.QueryRow(`SELECT title FROM media WHERE id='movie:10'`).Scan(&title); err != nil || title != "Broken" {
		t.Fatalf("title = %q: %v", title, err)
	}
}
