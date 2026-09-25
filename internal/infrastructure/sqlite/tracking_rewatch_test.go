package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestTracking_GivenRewatches_WhenMarkedUnwatched_ThenAllPlaysAndTheirFeedEventsAreRemoved(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	aliceID := insertTestUser(t, store.DB, "alice", "Alice", "instance")
	bobID := insertTestUser(t, store.DB, "bob", "Bob", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES
		('movie:10','movie',10,'Example Movie',?,?),('tv:42','tv',42,'Example Show',?,?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season 1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:42:episode:101','tv:42','tv:42:season:1',1,1,'Pilot')`); err != nil {
		t.Fatal(err)
	}
	movieID := "movie:10"
	for _, play := range []tracking.Play{
		{UserID: aliceID, MediaID: &movieID, WatchedAt: time.Now().UTC(), Source: "web"},
		{UserID: aliceID, MediaID: &movieID, WatchedAt: time.Now().UTC(), Source: "web"},
		{UserID: bobID, MediaID: &movieID, WatchedAt: time.Now().UTC(), Source: "web"},
	} {
		if _, err := store.CreatePlay(ctx, play); err != nil {
			t.Fatal(err)
		}
	}
	episodeID := "tv:42:episode:101"
	for range 2 {
		if _, err := store.CreateBulkPlays(ctx, []tracking.Play{{UserID: aliceID, EpisodeID: &episodeID, WatchedAt: time.Now().UTC(), Source: "web"}}); err != nil {
			t.Fatal(err)
		}
	}

	var rewatches int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE user_id=? AND kind='rewatch'`, aliceID).Scan(&rewatches); err != nil {
		t.Fatal(err)
	}
	if rewatches != 2 {
		t.Fatalf("rewatch activity count=%d, want 2 (movie and episode)", rewatches)
	}
	if err := store.DeleteEpisodePlays(ctx, aliceID, []string{episodeID}); err != nil {
		t.Fatalf("mark episode unwatched: %v", err)
	}
	if err := store.DeleteMediaPlays(ctx, aliceID, movieID); err != nil {
		t.Fatalf("mark movie unwatched: %v", err)
	}
	var alicePlays, aliceEvents, bobPlays int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=?`, aliceID).Scan(&alicePlays); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE user_id=? AND kind IN ('watch','rewatch','bulk_watch')`, aliceID).Scan(&aliceEvents); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=?`, bobID).Scan(&bobPlays); err != nil {
		t.Fatal(err)
	}
	if alicePlays != 0 || aliceEvents != 0 || bobPlays != 1 {
		t.Fatalf("remaining plays/events alice=%d/%d bob=%d, want 0/0 and 1", alicePlays, aliceEvents, bobPlays)
	}
}
