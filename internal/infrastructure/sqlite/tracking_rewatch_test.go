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
	for _, user := range []struct{ id, visibility string }{{"alice", "instance"}, {"bob", "private"}} {
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
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:42:episode:101','tv:42','tv:42:season:1',1,1,'Pilot')`); err != nil {
		t.Fatal(err)
	}
	movieID := "movie:10"
	for _, play := range []tracking.Play{
		{ID: "movie-first", UserID: "alice", MediaID: &movieID, WatchedAt: time.Now().UTC(), Source: "web"},
		{ID: "movie-again", UserID: "alice", MediaID: &movieID, WatchedAt: time.Now().UTC(), Source: "web"},
		{ID: "bob-movie", UserID: "bob", MediaID: &movieID, WatchedAt: time.Now().UTC(), Source: "web"},
	} {
		if err := store.CreatePlay(ctx, play); err != nil {
			t.Fatal(err)
		}
	}
	episodeID := "tv:42:episode:101"
	for _, playID := range []string{"episode-first", "episode-again"} {
		if err := store.CreateBulkPlays(ctx, []tracking.Play{{ID: playID, UserID: "alice", EpisodeID: &episodeID, WatchedAt: time.Now().UTC(), Source: "web"}}); err != nil {
			t.Fatal(err)
		}
	}

	var rewatches int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE user_id='alice' AND kind='rewatch'`).Scan(&rewatches); err != nil {
		t.Fatal(err)
	}
	if rewatches != 2 {
		t.Fatalf("rewatch activity count=%d, want 2 (movie and episode)", rewatches)
	}
	if err := store.DeleteEpisodePlays(ctx, "alice", []string{episodeID}); err != nil {
		t.Fatalf("mark episode unwatched: %v", err)
	}
	if err := store.DeleteMediaPlays(ctx, "alice", movieID); err != nil {
		t.Fatalf("mark movie unwatched: %v", err)
	}
	var alicePlays, aliceEvents, bobPlays int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id='alice'`).Scan(&alicePlays); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE user_id='alice' AND kind IN ('watch','rewatch','bulk_watch')`).Scan(&aliceEvents); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id='bob'`).Scan(&bobPlays); err != nil {
		t.Fatal(err)
	}
	if alicePlays != 0 || aliceEvents != 0 || bobPlays != 1 {
		t.Fatalf("remaining plays/events alice=%d/%d bob=%d, want 0/0 and 1", alicePlays, aliceEvents, bobPlays)
	}
}
