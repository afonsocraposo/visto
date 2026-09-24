package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

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
