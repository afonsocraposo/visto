package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/feed"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/profile"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestFeed_GivenPrivateAndOptedInActivity_WhenListed_ThenPrivateEventsStayHiddenAndBulkIsAggregated(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, user := range []struct{ id, visibility string }{{"private-user", "private"}, {"family-user", "instance"}} {
		if _, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(?,?,?,'hash','user',?,?)`, user.id, user.id, user.id, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB.Exec(`INSERT INTO user_settings(user_id,timezone,activity_visibility,created_at,updated_at) VALUES(?,'UTC',?,?,?)`, user.id, user.visibility, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES
		('movie:10','movie',10,'Example Movie',?,?),('tv:42','tv',42,'Example Show',?,?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season 1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES
		('tv:42:episode:101','tv:42','tv:42:season:1',1,1,'Pilot'),
		('tv:42:episode:102','tv:42','tv:42:season:1',1,2,'Second Episode')`); err != nil {
		t.Fatal(err)
	}

	// Given one private movie watch and one opted-in rewatch sequence.
	movieID := "movie:10"
	privatePlay := tracking.Play{ID: "private-play", UserID: "private-user", MediaID: &movieID, WatchedAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC), Source: "web"}
	if err := store.CreatePlay(ctx, privatePlay); err != nil {
		t.Fatal(err)
	}
	for _, play := range []tracking.Play{
		{ID: "family-first-play", UserID: "family-user", MediaID: &movieID, WatchedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), Source: "web"},
		{ID: "family-rewatch", UserID: "family-user", MediaID: &movieID, WatchedAt: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC), Source: "web"},
	} {
		if err := store.CreatePlay(ctx, play); err != nil {
			t.Fatal(err)
		}
	}
	episodeIDs := []string{"tv:42:episode:101", "tv:42:episode:102"}
	bulkPlays := []tracking.Play{
		{ID: "bulk-play-1", UserID: "family-user", EpisodeID: &episodeIDs[0], WatchedAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), Source: "web"},
		{ID: "bulk-play-2", UserID: "family-user", EpisodeID: &episodeIDs[1], WatchedAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), Source: "web"},
	}
	if err := store.CreateBulkPlays(ctx, bulkPlays); err != nil {
		t.Fatal(err)
	}
	// A first rating and a later rating change are separate eligible events.
	for index, rating := range []int{5, 4} {
		updatedAt := time.Date(2026, 9, 24, 12+index, 0, 0, 0, time.UTC)
		if err := store.UpsertItem(ctx, library.Item{
			UserID: "family-user", MediaID: movieID, Status: domain.WatchlistStatus,
			Rating: &rating, AddedAt: updatedAt, UpdatedAt: updatedAt,
		}); err != nil {
			t.Fatal(err)
		}
	}
	privateRating := 5
	if err := store.UpsertItem(ctx, library.Item{
		UserID: "private-user", MediaID: movieID, Status: domain.WatchlistStatus,
		Rating: &privateRating, AddedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	// When the instance feed is listed, private activity is absent and the
	// opted-in user's plays have distinct watch/rewatch and one aggregate item.
	page, err := feed.NewService(store).List(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	seenRatings := map[int]bool{}
	for _, item := range page.Items {
		if item.DisplayName == "private-user" {
			t.Fatalf("private activity leaked into the feed: %+v", item)
		}
		kinds[item.Kind]++
		if item.Kind == "bulk_watch" && (item.Count != 2 || item.Title != "Example Show") {
			t.Fatalf("bulk activity was not aggregated with its show and count: %+v", item)
		}
		if item.Kind == "rating" {
			if item.Title != "Example Movie" || item.Rating == nil {
				t.Fatalf("rating activity is missing its title or value: %+v", item)
			}
			seenRatings[*item.Rating] = true
		}
	}
	if kinds["watch"] != 1 || kinds["rewatch"] != 1 || kinds["bulk_watch"] != 1 || kinds["rating"] != 2 || !seenRatings[5] || !seenRatings[4] || len(page.Items) != 5 {
		t.Fatalf("feed activity kinds=%v items=%+v", kinds, page.Items)
	}

	// When the account changes to private, its old activity is removed too.
	settings := profile.Settings{ActivityVisibility: profile.PrivateVisibility, Timezone: "UTC"}
	if err := store.SetSettings(ctx, "family-user", settings); err != nil {
		t.Fatal(err)
	}
	page, err = feed.NewService(store).List(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("activity remained after visibility changed to private: %+v", page.Items)
	}
}

const testTimestamp = "2026-09-24T00:00:00Z"
