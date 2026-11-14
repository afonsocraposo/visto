package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/ratings"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

type ratingsProvider struct {
	resolves, calls int
	episodeResolves int
	fail            bool
}

func (p *ratingsProvider) EpisodeIMDbID(_ context.Context, _ int64, season, episode int) (string, error) {
	p.episodeResolves++
	return fmt.Sprintf("tt%07d", 1000000+season*100+episode), nil
}

func (p *ratingsProvider) IMDbID(context.Context, domain.MediaType, int64) (string, error) {
	p.resolves++
	return "tt0111161", nil
}
func (p *ratingsProvider) Ratings(context.Context, string) (ratings.Scores, error) {
	p.calls++
	if p.fail {
		return ratings.Scores{}, errors.New("offline")
	}
	imdb, rotten := 9.3, 91
	return ratings.Scores{IMDb: &imdb, RottenTomatoes: &rotten}, nil
}

func TestExternalRatingsPersistAndRefreshInBackground(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "visto.db")
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { store.Close() }()
	provider := &ratingsProvider{}
	service := ratings.New(store, provider, provider)
	first, err := service.Get(ctx, domain.MovieMediaType, 10)
	if err != nil || first.IMDb == nil || *first.IMDb != 9.3 || first.RottenTomatoes == nil || *first.RottenTomatoes != 91 || provider.calls != 1 {
		t.Fatalf("first=%+v err=%v calls=%d", first, err, provider.calls)
	}
	// Reopen the database and recreate the service to verify durable caching.
	store.Close()
	store, err = sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	service = ratings.New(store, provider, provider)
	if _, err := service.Get(ctx, domain.MovieMediaType, 10); err != nil || provider.calls != 1 {
		t.Fatalf("cache after restart: %v, calls=%d", err, provider.calls)
	}
	old := first.Scores
	old.FetchedAt = time.Now().UTC().Add(-2 * 24 * time.Hour)
	if err := store.SaveExternalRatings(ctx, ratings.Title{Type: domain.MovieMediaType, TMDBID: 10}, old); err != nil {
		t.Fatal(err)
	}
	if err := service.Refresh(ctx); err != nil || provider.calls != 1 {
		t.Fatalf("scores younger than a week refreshed: %v", err)
	}
	old.FetchedAt = time.Now().UTC().Add(-8 * 24 * time.Hour)
	if err := store.SaveExternalRatings(ctx, ratings.Title{Type: domain.MovieMediaType, TMDBID: 10}, old); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(ctx, domain.MovieMediaType, 10); err != nil || provider.calls != 1 {
		t.Fatalf("page view refreshed scores: %v, calls=%d", err, provider.calls)
	}
	if err := service.Refresh(ctx); err != nil || provider.calls != 2 || provider.resolves != 1 {
		t.Fatalf("background refresh: %v provider=%+v", err, provider)
	}
	if err := service.Refresh(ctx); err != nil || provider.calls != 2 {
		t.Fatalf("fresh scores refreshed again: %v", err)
	}
	// A failure must retain the last successful scores and their timestamp.
	if err := store.SaveExternalRatings(ctx, ratings.Title{Type: domain.MovieMediaType, TMDBID: 10}, old); err != nil {
		t.Fatal(err)
	}
	provider.fail = true
	if err := service.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	cached, found, err := store.GetExternalRatings(ctx, ratings.Title{Type: domain.MovieMediaType, TMDBID: 10})
	if err != nil || !found || cached.IMDb == nil || !cached.FetchedAt.Equal(old.FetchedAt) {
		t.Fatalf("stale cache lost: %+v %v", cached, err)
	}
	calls := provider.calls
	if err := service.Refresh(ctx); err != nil || provider.calls != calls {
		t.Fatalf("failure cooldown ignored: %v", err)
	}
	var disabled *ratings.Service
	result, err := disabled.Get(ctx, domain.MovieMediaType, 10)
	if err != nil || result.Enabled {
		t.Fatalf("disabled ratings=%+v %v", result, err)
	}
}

func TestEpisodeScoresHaveIndependentDurableCachesAndWeeklyRefresh(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	provider := &ratingsProvider{}
	service := ratings.New(store, provider, provider)
	series, err := service.Get(ctx, domain.TVMediaType, 42)
	if err != nil {
		t.Fatal(err)
	}
	for _, numbers := range [][2]int{{1, 1}, {1, 2}, {0, 1}} {
		result, err := service.GetEpisode(ctx, 42, numbers[0], numbers[1])
		if err != nil || result.IMDbID == series.IMDbID || result.IMDb == nil || result.RottenTomatoes == nil {
			t.Fatalf("episode scores=%+v err=%v", result, err)
		}
	}
	if provider.calls != 4 || provider.episodeResolves != 3 {
		t.Fatalf("provider=%+v", provider)
	}
	// Recreating the service still reads the same episode snapshot from SQLite.
	service = ratings.New(store, provider, provider)
	first, err := service.GetEpisode(ctx, 42, 1, 1)
	if err != nil || first.IMDbID != "tt1000101" || provider.calls != 4 {
		t.Fatalf("cached episode=%+v %v", first, err)
	}
	title := ratings.Title{Type: domain.TVMediaType, TMDBID: 42, SeasonNumber: 1, EpisodeNumber: 1}
	old := first.Scores
	old.FetchedAt = time.Now().UTC().Add(-8 * 24 * time.Hour)
	if err := store.SaveExternalRatings(ctx, title, old); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetEpisode(ctx, 42, 1, 1); err != nil || provider.calls != 4 {
		t.Fatalf("page view fetched expired score: %v", err)
	}
	if err := service.Refresh(ctx); err != nil || provider.calls != 5 || provider.episodeResolves != 3 {
		t.Fatalf("episode background refresh: %v provider=%+v", err, provider)
	}
	for _, numbers := range [][2]int{{-1, 1}, {1, 0}} {
		if _, err := service.GetEpisode(ctx, 42, numbers[0], numbers[1]); err == nil {
			t.Fatal("invalid episode accepted")
		}
	}
}
