package tmdb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestFindByTVDB(t *testing.T) {
	client, err := New("test-key", &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/3/find/123" || request.URL.Query().Get("external_source") != "tvdb_id" {
			t.Errorf("unexpected lookup: %s", request.URL.String())
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"tv_results":[{"id":42,"name":"Example"}]}`)), Header: make(http.Header), Request: request}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	id, err := client.FindByTVDB(context.Background(), "tv", "123")
	if err != nil || id != 42 {
		t.Fatalf("resolved ID=%d, error=%v", id, err)
	}
}

func TestFindEpisodeByExternalID(t *testing.T) {
	var lookedUp string
	client, err := New("test-key", &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		lookedUp = request.URL.Path + "?" + request.URL.Query().Get("external_source")
		body := `{"tv_episode_results":[{"id":9004,"show_id":100,"season_number":2,"episode_number":4}]}`
		if strings.Contains(request.URL.Path, "/none") {
			body = `{"tv_episode_results":[]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: request}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	location, err := client.FindEpisodeByExternalID(context.Background(), "tvdb_id", "11861026")
	if err != nil || location.ShowTMDBID != 100 || location.SeasonNumber != 2 || location.EpisodeNumber != 4 {
		t.Fatalf("location=%#v error=%v; want show 100 season 2 episode 4", location, err)
	}
	if lookedUp != "/3/find/11861026?tvdb_id" {
		t.Fatalf("lookup = %q, want a TVDB find", lookedUp)
	}
	if _, err := client.FindEpisodeByExternalID(context.Background(), "imdb_id", "none"); err == nil {
		t.Fatal("an ID with no episode match must fail")
	}
	for _, invalid := range [][2]string{{"tmdb_id", "1"}, {"tvdb_id", ""}, {"tvdb_id", "1/2"}} {
		if _, err := client.FindEpisodeByExternalID(context.Background(), invalid[0], invalid[1]); err == nil {
			t.Fatalf("source %q id %q must be rejected", invalid[0], invalid[1])
		}
	}
}

func TestDetailCaches_GivenMoreThanTheLimit_StayBounded(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"id":42,"title":"Example","name":"Example","season_number":1,"episode_number":1}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: request}, nil
	})
	client, err := New("test-key", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	for id := int64(1); id <= maxShowCacheEntries; id++ {
		client.movieCache[id+100] = cachedMovie{expiresAt: expires}
	}
	for id := 0; id < maxShowCacheEntries*10; id++ {
		client.episodeCache[fmt.Sprint("old:", id)] = cachedEpisode{expiresAt: expires}
	}
	if _, err := client.Movie(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Episode(context.Background(), 42, 1, 1); err != nil {
		t.Fatal(err)
	}
	if len(client.movieCache) != maxShowCacheEntries || len(client.episodeCache) != maxShowCacheEntries*10 {
		t.Fatalf("cache sizes: movies=%d episodes=%d", len(client.movieCache), len(client.episodeCache))
	}
	if _, ok := client.movieCache[42]; !ok {
		t.Fatal("new movie was evicted")
	}
	if _, ok := client.episodeCache["42:1:1"]; !ok {
		t.Fatal("new episode was evicted")
	}
}

func TestLatestRegularSeasonNumber_GivenSpecialsAndRegularSeasons_ReturnsHighestRegularSeason(t *testing.T) {
	got, ok := latestRegularSeasonNumber([]domain.TVSeasonMetadata{{Number: 0}, {Number: 1}, {Number: 7}, {Number: 3}})
	if !ok || got != 7 {
		t.Fatalf("latest regular season = %d, %t; want 7, true", got, ok)
	}
	if _, ok := latestRegularSeasonNumber([]domain.TVSeasonMetadata{{Number: 0}}); ok {
		t.Fatal("specials-only metadata should not trigger a regular season fetch")
	}
}

func TestRelated_GivenMovieRecommendations_WhenRequestedTwice_ThenItFiltersAndCachesResults(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/3/movie/42/recommendations" {
			t.Errorf("request path=%q, want movie recommendations", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":1,"results":[{"id":42,"title":"Current title","adult":false},{"id":43,"title":"Adult title","adult":true},{"id":44,"title":"Recommended","original_title":"Recommended","overview":"A related film.","release_date":"2025-01-01","poster_path":"/recommended.jpg","backdrop_path":"/backdrop.jpg","original_language":"en","adult":false}]}`))
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)

	first, err := client.Related(context.Background(), domain.MovieMediaType, 42)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Related(context.Background(), domain.MovieMediaType, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].TMDBID != 44 || first[0].Type != domain.MovieMediaType || first[0].PosterPath != "/recommended.jpg" {
		t.Fatalf("related media=%+v, want only the valid recommended movie", first)
	}
	if len(second) != 1 || calls.Load() != 1 {
		t.Fatalf("cached media=%+v requests=%d, want one cached request", second, calls.Load())
	}
}

func TestRelated_GivenTVRecommendations_WhenRequested_ThenItMapsTVFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":1,"results":[{"id":51,"name":"Related Show","original_name":"Related Show Original","overview":"A related series.","first_air_date":"2024-04-01","poster_path":"/show.jpg","backdrop_path":"/show-backdrop.jpg","original_language":"fr"}]}`))
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)

	results, err := client.Related(context.Background(), domain.TVMediaType, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Type != domain.TVMediaType || results[0].Title != "Related Show" || results[0].ReleaseDate != "2024-04-01" {
		t.Fatalf("related media=%+v, want mapped TV recommendation", results)
	}
}

func TestEmptyTMDBLists_RemainNonNilAfterCaching(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/person/"):
			_, _ = w.Write([]byte(`{"id":42,"name":"Example","combined_credits":{"cast":[]}}`))
		default:
			_, _ = w.Write([]byte(`{"page":1,"results":[],"total_results":0,"total_pages":0}`))
		}
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)
	for attempt := range 2 {
		for name, fetch := range map[string]func() ([]domain.MediaSearchResult, error){
			"search": func() ([]domain.MediaSearchResult, error) { return client.Search(context.Background(), "sopramos", "") },
			"search tv": func() ([]domain.MediaSearchResult, error) {
				return client.SearchType(context.Background(), domain.TVMediaType, "sopramos", "")
			},
			"search movie": func() ([]domain.MediaSearchResult, error) {
				return client.SearchType(context.Background(), domain.MovieMediaType, "sopramos", "")
			},
			"related": func() ([]domain.MediaSearchResult, error) {
				return client.Related(context.Background(), domain.TVMediaType, 42)
			},
			"trending": func() ([]domain.MediaSearchResult, error) { return client.Trending(context.Background(), "tv", "week") },
		} {
			results, err := fetch()
			if err != nil || results == nil || len(results) != 0 {
				t.Fatalf("%s attempt %d: results=%v err=%v, want non-nil empty slice", name, attempt, results, err)
			}
		}
		person, err := client.Person(context.Background(), 42)
		if err != nil || person.Credits == nil || len(person.Credits) != 0 {
			t.Fatalf("person attempt %d: credits=%v err=%v, want non-nil empty slice", attempt, person.Credits, err)
		}
	}
}

func TestSearch_GivenTMDBReturns429_WhenRetryAfterExpires_ThenItRetriesWithinTheConfiguredCap(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"status_code":25,"status_message":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":1,"results":[],"total_results":0,"total_pages":0}`))
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)
	results, err := client.Search(context.Background(), "example", "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 || calls.Load() != 2 {
		t.Fatalf("results=%v calls=%d, want empty results after one retry", results, calls.Load())
	}
}

func TestPerson_GivenMixedDuplicateCredits_WhenLoadedTwice_ThenItSortsDeduplicatesAndCachesThem(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/3/person/123" || r.URL.Query().Get("append_to_response") != "combined_credits" {
			t.Errorf("request URL=%s, want combined person details", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"name":"Example Actor","biography":"A short biography.","profile_path":"/actor.jpg","combined_credits":{"cast":[{"id":8,"media_type":"tv","name":"Popular Show","original_name":"Popular Show","character":"Lead","first_air_date":"2020-01-01","poster_path":"/show.jpg","popularity":90},{"id":7,"media_type":"movie","title":"Popular Film","original_title":"Popular Film","character":"Self","release_date":"2021-01-01","poster_path":"/film.jpg","popularity":120},{"id":8,"media_type":"tv","name":"Popular Show","character":"Lead","popularity":90},{"id":99,"media_type":"movie","title":"Adult title","adult":true,"popularity":200}]}}`))
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)

	person, err := client.Person(context.Background(), 123)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Person(context.Background(), 123); err != nil {
		t.Fatal(err)
	}
	if person.Name != "Example Actor" || len(person.Credits) != 2 {
		t.Fatalf("person=%+v, want two non-adult unique credits", person)
	}
	if person.Credits[0].Title != "Popular Film" || person.Credits[1].Title != "Popular Show" {
		t.Fatalf("credits order = %+v, want popularity descending", person.Credits)
	}
	if calls.Load() != 1 {
		t.Fatalf("provider calls=%d, want one cached request", calls.Load())
	}
}

func TestSearch_GivenConcurrentIdenticalQueries_WhenRequested_ThenItCoalescesProviderCalls(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(80 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":1,"results":[],"total_results":0,"total_pages":0}`))
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)
	start := make(chan struct{})
	errors := make(chan error, 2)
	for range 2 {
		go func() { <-start; _, err := client.Search(context.Background(), "same", "en-US"); errors <- err }()
	}
	close(start)
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("provider calls=%d, want 1", calls.Load())
	}
}

func TestSearch_GivenDifferentRequestsReceive429_WhenRetried_ThenTheInstancePausePreventsRetryBursts(t *testing.T) {
	var mu sync.Mutex
	callsByQuery := map[string]int{}
	var limitedRequests atomic.Int32
	var initialArrivals atomic.Int32
	bothArrived := make(chan struct{})
	bothLimited := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		mu.Lock()
		callsByQuery[query]++
		callNumber := callsByQuery[query]
		mu.Unlock()
		if query != "third" && callNumber == 1 {
			if initialArrivals.Add(1) == 2 {
				close(bothArrived)
			}
			select {
			case <-bothArrived:
			case <-time.After(5 * time.Second):
				http.Error(w, "second request did not arrive", http.StatusRequestTimeout)
				return
			}
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"status_code":25,"status_message":"rate limited"}`))
			if limitedRequests.Add(1) == 2 {
				close(bothLimited)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":1,"results":[],"total_results":0,"total_pages":0}`))
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)

	start := make(chan struct{})
	results := make(chan error, 3)
	for _, query := range []string{"first", "second"} {
		go func(query string) {
			<-start
			_, err := client.Search(context.Background(), query, "en-US")
			results <- err
		}(query)
	}
	close(start)
	select {
	case <-bothLimited:
	case <-time.After(5 * time.Second):
		t.Fatal("both initial requests did not receive 429")
	}
	deadline := time.Now().Add(time.Second)
	for {
		client.mu.Lock()
		paused := client.pausedUntil.After(time.Now())
		client.mu.Unlock()
		if paused {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("429 did not establish the shared provider pause")
		}
		time.Sleep(time.Millisecond)
	}
	go func() {
		_, err := client.Search(context.Background(), "third", "en-US")
		results <- err
	}()

	// A distinct new request must wait too; per-query backoff alone would not
	// prevent it from reaching the provider during this window.
	time.Sleep(250 * time.Millisecond)
	mu.Lock()
	initialCallCount := callsByQuery["first"] + callsByQuery["second"] + callsByQuery["third"]
	mu.Unlock()
	if initialCallCount != 2 {
		t.Fatalf("provider calls during shared pause=%d, want exactly 2 initial calls", initialCallCount)
	}

	for range 3 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(6 * time.Second):
			t.Fatal("limited search did not finish after its bounded retry")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if callsByQuery["first"] != 2 || callsByQuery["second"] != 2 || callsByQuery["third"] != 1 {
		t.Fatalf("provider calls by query=%v, want one initial request and retry for limited queries, one for new query", callsByQuery)
	}
}

func TestShowCache_GivenMoreThanTheConfiguredLimit_WhenShowsAreCached_ThenMemoryUseStaysBounded(t *testing.T) {
	client := &Client{showCache: make(map[int64]cachedShow)}
	expiresAt := time.Now().Add(time.Hour)
	for tmdbID := int64(1); tmdbID <= maxShowCacheEntries+1; tmdbID++ {
		client.mu.Lock()
		client.cacheShowLocked(tmdbID, domain.TVShowMetadata{TMDBID: tmdbID, Name: "Example"}, expiresAt)
		client.mu.Unlock()
	}

	if len(client.showCache) != maxShowCacheEntries {
		t.Fatalf("cached shows=%d, want max %d", len(client.showCache), maxShowCacheEntries)
	}
	if _, ok := client.showCache[maxShowCacheEntries+1]; !ok {
		t.Fatal("newly cached show should be retained")
	}
}

func TestSearchType_GivenAMediaType_ThenItUsesThatTMDBSearchAndCachesItSeparately(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/3/search/tv":
			_, _ = w.Write([]byte(`{"page":1,"results":[{"id":1,"name":"Show","original_name":"Show","first_air_date":"2020-01-01"}]}`))
		case "/3/search/movie":
			_, _ = w.Write([]byte(`{"page":1,"results":[{"id":2,"title":"Film","original_title":"Film","release_date":"2021-01-01"}]}`))
		default:
			_, _ = w.Write([]byte(`{"page":1,"results":[{"id":1,"media_type":"tv","name":"Show"},{"id":2,"media_type":"movie","title":"Film"}]}`))
		}
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)
	for range 2 {
		shows, err := client.SearchType(context.Background(), domain.TVMediaType, "same", "en-US")
		if err != nil || len(shows) != 1 || shows[0].Type != domain.TVMediaType || shows[0].Title != "Show" || shows[0].ReleaseDate != "2020-01-01" {
			t.Fatalf("shows=%v err=%v", shows, err)
		}
		movies, err := client.SearchType(context.Background(), domain.MovieMediaType, "same", "en-US")
		if err != nil || len(movies) != 1 || movies[0].Type != domain.MovieMediaType || movies[0].Title != "Film" {
			t.Fatalf("movies=%v err=%v", movies, err)
		}
		mixed, err := client.Search(context.Background(), "same", "en-US")
		if err != nil || len(mixed) != 2 {
			t.Fatalf("mixed=%v err=%v", mixed, err)
		}
	}
	if want := []string{"/3/search/tv", "/3/search/movie", "/3/search/multi"}; !slices.Equal(paths, want) {
		t.Fatalf("paths=%v, want %v (each type fetched once, then cached)", paths, want)
	}
	if _, err := client.SearchType(context.Background(), "person", "same", ""); err == nil {
		t.Fatal("expected an error for an unsupported media type")
	}
}

func TestShowSummary_GivenAggregateCredits_WhenFetched_ThenCastSpansTheWholeSeriesInOrder(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/3/tv/2316" || r.URL.Query().Get("append_to_response") != "aggregate_credits" {
			t.Errorf("request URL=%s, want TV details with aggregate credits", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":2316,"name":"The Office","status":"Ended","seasons":[{"id":1,"season_number":1,"episode_count":6}],
			"credits":{"cast":[{"id":99,"name":"Final Season Only","character":"Newcomer","order":0}]},
			"aggregate_credits":{"cast":[
				{"id":3,"name":"Ed Helms","profile_path":"/ed.jpg","order":2,"total_episode_count":146,"roles":[{"character":"Andy Bernard","episode_count":146}]},
				{"id":1,"name":"Steve Carell","profile_path":"/steve.jpg","order":0,"total_episode_count":146,"roles":[{"character":"Michael Scott","episode_count":146}]},
				{"id":4,"name":"Multi Role","order":2,"total_episode_count":160,"roles":[{"character":"Minor","episode_count":3},{"character":"Main","episode_count":150},{"character":"Main","episode_count":7},{"character":" ","episode_count":1},{"character":"Third","episode_count":2}]},
				{"id":2,"name":"Rainn Wilson","profile_path":"/rainn.jpg","order":1,"total_episode_count":201,"roles":[{"character":"Dwight Schrute","episode_count":201}]}
			]}}`))
	}))
	defer server.Close()
	client, err := New("test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.client.GetBaseURL()
	client.client.SetCustomBaseURL(server.URL + "/3")
	defer client.client.SetCustomBaseURL(previous)

	show, err := client.ShowSummary(context.Background(), 2316)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.TVCastMember{
		{ID: 1, Name: "Steve Carell", Character: "Michael Scott", ProfilePath: "/steve.jpg"},
		{ID: 2, Name: "Rainn Wilson", Character: "Dwight Schrute", ProfilePath: "/rainn.jpg"},
		{ID: 4, Name: "Multi Role", Character: "Main / Minor"},
		{ID: 3, Name: "Ed Helms", Character: "Andy Bernard", ProfilePath: "/ed.jpg"},
	}
	if !slices.Equal(show.Cast, want) {
		t.Fatalf("cast=%+v, want aggregate cast %+v", show.Cast, want)
	}
	if show.Name != "The Office" || len(show.Seasons) != 1 {
		t.Fatalf("show=%+v, want details still mapped", show)
	}
	if _, err := client.ShowSummary(context.Background(), 2316); err != nil || calls.Load() != 1 {
		t.Fatalf("calls=%d err=%v, want aggregate cast cached with the summary", calls.Load(), err)
	}
}
