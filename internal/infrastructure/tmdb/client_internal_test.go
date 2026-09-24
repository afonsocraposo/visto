package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

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
	bothLimited := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		mu.Lock()
		callsByQuery[query]++
		callNumber := callsByQuery[query]
		mu.Unlock()
		if query != "third" && callNumber == 1 {
			w.Header().Set("Retry-After", "1")
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
	case <-time.After(2 * time.Second):
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
		case <-time.After(4 * time.Second):
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
