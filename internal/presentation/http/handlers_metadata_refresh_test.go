package httpserver_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

type refreshMetadataProvider struct {
	mu                                      sync.Mutex
	summaryCalls, seasonCalls, episodeCalls int
	episode                                 domain.TVEpisodeMetadata
	show                                    domain.TVShowMetadata
	movie                                   domain.MovieMetadata
	err                                     error
	called                                  int
}

func (*refreshMetadataProvider) Search(context.Context, string, string) ([]domain.MediaSearchResult, error) {
	return nil, nil
}

func (provider *refreshMetadataProvider) Show(context.Context, int64) (domain.TVShowMetadata, error) {
	provider.mu.Lock()
	provider.called++
	provider.mu.Unlock()
	return provider.show, provider.err
}

func (provider *refreshMetadataProvider) Movie(context.Context, int64) (domain.MovieMetadata, error) {
	provider.mu.Lock()
	provider.called++
	provider.mu.Unlock()
	return provider.movie, provider.err
}

type metadataRefreshFixture struct {
	path     string
	store    *sqlite.Store
	provider *refreshMetadataProvider
	handler  http.Handler
}

func newMetadataRefreshFixture(t *testing.T) metadataRefreshFixture {
	t.Helper()
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "visto.db")
	store, err := sqlite.Open(ctx, databasePath)
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
	handler := httpserver.New(auth.NewService(store), provider, "", library.NewService(store), nil, nil, nil, nil, watch.NewService(store, provider)).Handler()
	return metadataRefreshFixture{path: databasePath, store: store, provider: provider, handler: handler}
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

func (provider *refreshMetadataProvider) ShowSummary(context.Context, int64) (domain.TVShowMetadata, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.summaryCalls++
	show := provider.show
	show.Seasons = append([]domain.TVSeasonMetadata(nil), show.Seasons...)
	for i := range show.Seasons {
		show.Seasons[i].EpisodeCount = len(show.Seasons[i].Episodes)
	}
	// Deliberately retain episodes to exercise protection against cached full-show payloads.
	return show, provider.err
}
func (provider *refreshMetadataProvider) Season(_ context.Context, _ int64, number int) (domain.TVSeasonMetadata, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.seasonCalls++
	for _, season := range provider.show.Seasons {
		if season.Number == number {
			return season, provider.err
		}
	}
	return domain.TVSeasonMetadata{}, errors.New("missing season")
}
func (provider *refreshMetadataProvider) Episode(context.Context, int64, int, int) (domain.TVEpisodeMetadata, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.episodeCalls++
	return provider.episode, provider.err
}
func (fixture metadataRefreshFixture) get(path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "viewer-session"})
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	return response
}

func TestSavedDetailGETsRefreshOnlyStaleResourceAndPreserveUserData(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	ctx := context.Background()
	if _, err := fixture.store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at,note,notifications_enabled) VALUES('movie-item',2,'movie:10','watchlist',?,?,'keep note',0)`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.DB.Exec(`INSERT INTO episode_notes(user_id,episode_id,note,updated_at) VALUES(2,'tv:2316:episode:1','episode note','2026-01-01'); INSERT INTO episode_ratings(user_id,episode_id,rating,created_at,updated_at) VALUES(2,'tv:2316:episode:1',4,'2026-01-01','2026-01-01'); INSERT INTO notification_deliveries(user_id,episode_id,state,attempted_at) VALUES(2,'tv:2316:episode:1','sent','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	// Missing season and details timestamps must be stale.
	fixture.provider.episode = domain.TVEpisodeMetadata{TMDBID: 1001, SeasonNumber: 1, EpisodeNumber: 1, Name: "Detailed Pilot", VoteAverage: 8.5, ProductionCode: "P1", GuestStars: []domain.TVCastMember{{ID: 7, Name: "Guest"}}, Crew: []domain.TVCrewMember{{ID: 8, Name: "Director", Job: "Director"}}}
	for _, path := range []string{"/api/v1/movies/10", "/api/v1/shows/2316"} {
		response := fixture.get(path)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
		var entry library.Entry
		if err := json.Unmarshal(response.Body.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Media.Overview == "" {
			t.Fatalf("stale response: %s", response.Body.String())
		}
	}
	if fixture.provider.summaryCalls != 1 || fixture.provider.seasonCalls != 0 || fixture.provider.called != 1 {
		t.Fatalf("show fetched more than summary: %#v", fixture.provider)
	}
	var count int
	if err := fixture.store.DB.QueryRow(`SELECT COUNT(*) FROM episodes WHERE show_id='tv:2316'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("summary imported episodes: count=%d err=%v", count, err)
	}
	var seasonStamp *string
	if err := fixture.store.DB.QueryRow(`SELECT metadata_updated_at FROM seasons WHERE id='tv:2316:season:1'`).Scan(&seasonStamp); err != nil || seasonStamp != nil {
		t.Fatalf("summary marked season fresh: %v %v", seasonStamp, err)
	}
	episodePath := "/api/v1/discover/shows/2316/seasons/1/episodes/1"
	response := fixture.get(episodePath)
	if response.Code != http.StatusOK {
		t.Fatalf("episode: %d %s", response.Code, response.Body.String())
	}
	var details map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &details); err != nil {
		t.Fatal(err)
	}
	if details["name"] != "Detailed Pilot" || details["production_code"] != "P1" || details["vote_average"] != 8.5 {
		t.Fatalf("details=%v", details)
	}
	var raw, stamp string
	if err := fixture.store.DB.QueryRow(`SELECT raw_metadata,details_updated_at FROM episodes WHERE id='tv:2316:episode:1'`).Scan(&raw, &stamp); err != nil {
		t.Fatal(err)
	}
	response = fixture.get("/api/v1/seasons/tv:2316:season:1/episodes")
	if response.Code != http.StatusOK {
		t.Fatalf("season: %d %s", response.Code, response.Body.String())
	}
	var rawAfter, stampAfter string
	if err := fixture.store.DB.QueryRow(`SELECT raw_metadata,details_updated_at FROM episodes WHERE id='tv:2316:episode:1'`).Scan(&rawAfter, &stampAfter); err != nil || rawAfter != raw || stampAfter != stamp {
		t.Fatalf("season overwrote details: %v", err)
	}
	for _, path := range []string{"/api/v1/movies/10", "/api/v1/shows/2316", "/api/v1/seasons/tv:2316:season:1/episodes", episodePath} {
		if response := fixture.get(path); response.Code != http.StatusOK {
			t.Fatalf("fresh %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	if fixture.provider.called != 1 || fixture.provider.summaryCalls != 1 || fixture.provider.seasonCalls != 1 || fixture.provider.episodeCalls != 1 {
		t.Fatalf("fresh metadata requested TMDB: %#v", fixture.provider)
	}
	var note string
	var enabled bool
	if err := fixture.store.DB.QueryRow(`SELECT note,notifications_enabled FROM user_media WHERE id='movie-item'`).Scan(&note, &enabled); err != nil || note != "keep note" || enabled {
		t.Fatalf("lost user preferences: %q %v %v", note, enabled, err)
	}
	if err := fixture.store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE episode_id='tv:2316:episode:1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("lost plays: %d %v", count, err)
	}
	var episodeNote, deliveryState string
	var episodeRating int
	if err := fixture.store.DB.QueryRow(`SELECT n.note,r.rating,d.state FROM episode_notes n JOIN episode_ratings r ON r.user_id=n.user_id AND r.episode_id=n.episode_id JOIN notification_deliveries d ON d.user_id=n.user_id AND d.episode_id=n.episode_id WHERE n.episode_id='tv:2316:episode:1'`).Scan(&episodeNote, &episodeRating, &deliveryState); err != nil || episodeNote != "episode note" || episodeRating != 4 || deliveryState != "sent" {
		t.Fatalf("lost episode user data: note=%q rating=%d delivery=%q err=%v", episodeNote, episodeRating, deliveryState, err)
	}
	// A new service instance reads persisted details without a provider request.
	newService := watch.NewService(fixture.store, fixture.provider)
	episode, err := newService.EpisodeDetails(ctx, 2316, 1, 1, fixture.provider)
	if err != nil || episode.ProductionCode != "P1" || len(episode.Crew) != 1 || fixture.provider.episodeCalls != 1 {
		t.Fatalf("durable details=%+v err=%v", episode, err)
	}
	// Stale failures return all four last stored snapshots and leave timestamps unchanged.
	if _, err := fixture.store.DB.Exec(`UPDATE media SET metadata_updated_at='2000-01-01T00:00:00Z'; UPDATE seasons SET metadata_updated_at=NULL; UPDATE episodes SET details_updated_at=NULL`); err != nil {
		t.Fatal(err)
	}
	fixture.provider.err = errors.New("TMDB unavailable")
	for _, path := range []string{"/api/v1/movies/10", "/api/v1/shows/2316", "/api/v1/seasons/tv:2316:season:1/episodes", episodePath} {
		if response := fixture.get(path); response.Code != http.StatusOK {
			t.Fatalf("failed provider %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	var unchanged string
	if err := fixture.store.DB.QueryRow(`SELECT metadata_updated_at FROM media WHERE id='movie:10'`).Scan(&unchanged); err != nil || unchanged != "2000-01-01T00:00:00Z" {
		t.Fatalf("failure updated freshness: %q %v", unchanged, err)
	}
}

func TestConcurrentSavedDetailsCoalesceDatabaseRefreshes(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	if _, err := fixture.store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES('movie-item',2,'movie:10','watchlist','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	fixture.provider.episode = domain.TVEpisodeMetadata{TMDBID: 1001, SeasonNumber: 1, EpisodeNumber: 1, Name: "Pilot", ProductionCode: "P1"}
	for _, path := range []string{"/api/v1/movies/10", "/api/v1/shows/2316", "/api/v1/seasons/tv:2316:season:1/episodes", "/api/v1/discover/shows/2316/seasons/1/episodes/1"} {
		var workers sync.WaitGroup
		start := make(chan struct{})
		for range 20 {
			workers.Go(func() {
				<-start
				if response := fixture.get(path); response.Code != http.StatusOK {
					t.Errorf("%s: %d %s", path, response.Code, response.Body.String())
				}
			})
		}
		close(start)
		workers.Wait()
	}
	if fixture.provider.called != 1 || fixture.provider.summaryCalls != 1 || fixture.provider.seasonCalls != 1 || fixture.provider.episodeCalls != 1 {
		t.Fatalf("duplicate refreshes: movie=%d show=%d season=%d episode=%d", fixture.provider.called, fixture.provider.summaryCalls, fixture.provider.seasonCalls, fixture.provider.episodeCalls)
	}
}

func TestEpisodeDetailsSurviveDatabaseReopen(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	fixture.provider.episode = domain.TVEpisodeMetadata{TMDBID: 1001, SeasonNumber: 1, EpisodeNumber: 1, Name: "Pilot", VoteAverage: 8, ProductionCode: "P1", GuestStars: []domain.TVCastMember{{ID: 7, Name: "Guest"}}, Crew: []domain.TVCrewMember{{ID: 8, Name: "Director"}}}
	response := fixture.get("/api/v1/discover/shows/2316/seasons/1/episodes/1")
	if response.Code != http.StatusOK {
		t.Fatalf("details: %d %s", response.Code, response.Body.String())
	}
	if err := fixture.store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(context.Background(), fixture.path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	fixture.provider.err = errors.New("offline")
	episode, err := watch.NewService(reopened, fixture.provider).EpisodeDetails(context.Background(), 2316, 1, 1, fixture.provider)
	if err != nil || episode.ProductionCode != "P1" || episode.VoteAverage != 8 || len(episode.GuestStars) != 1 || len(episode.Crew) != 1 || fixture.provider.episodeCalls != 1 {
		t.Fatalf("reopened details=%+v err=%v calls=%d", episode, err, fixture.provider.episodeCalls)
	}
}

func TestUnsavedEpisodeDetailsDoNotCreateCatalogEntries(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	fixture.provider.episode = domain.TVEpisodeMetadata{TMDBID: 9001, SeasonNumber: 1, EpisodeNumber: 1, Name: "Unsaved", ProductionCode: "P9"}
	response := fixture.get("/api/v1/discover/shows/999/seasons/1/episodes/1")
	if response.Code != http.StatusOK {
		t.Fatalf("unsaved details: %d %s", response.Code, response.Body.String())
	}
	var count int
	if err := fixture.store.DB.QueryRow(`SELECT (SELECT COUNT(*) FROM media WHERE tmdb_id=999)+(SELECT COUNT(*) FROM episodes WHERE tmdb_id=9001)`).Scan(&count); err != nil || count != 0 || fixture.provider.episodeCalls != 1 {
		t.Fatalf("browsing persisted metadata: count=%d calls=%d err=%v", count, fixture.provider.episodeCalls, err)
	}
}
