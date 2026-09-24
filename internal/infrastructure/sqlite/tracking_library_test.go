package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestTracking_GivenNewAndExistingTitles_WhenPlaysAreRecorded_ThenItCreatesOrPreservesLibraryRelationships(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, userID := range []string{"alice", "bob"} {
		if _, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(?,?,?,'hash','user',?,?)`, userID, userID, userID, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB.Exec(`INSERT INTO user_settings(user_id,timezone,activity_visibility,created_at,updated_at) VALUES(?,'UTC','private',?,?)`, userID, testTimestamp, testTimestamp); err != nil {
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
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES('bob:movie:10','bob','movie:10','paused',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}

	// When Alice records a movie play, create the missing relationship as watching.
	movieID := "movie:10"
	if err := store.CreatePlay(ctx, tracking.Play{
		ID: "alice-movie-play", UserID: "alice", MediaID: &movieID,
		WatchedAt: time.Now().UTC(), Source: "web",
	}); err != nil {
		t.Fatal(err)
	}
	// A bulk episode action also creates the show relationship.
	episodeIDs := []string{"tv:42:episode:101", "tv:42:episode:102"}
	if err := store.CreateBulkPlays(ctx, []tracking.Play{
		{ID: "alice-episode-play-1", UserID: "alice", EpisodeID: &episodeIDs[0], WatchedAt: time.Now().UTC(), Source: "web"},
		{ID: "alice-episode-play-2", UserID: "alice", EpisodeID: &episodeIDs[1], WatchedAt: time.Now().UTC(), Source: "web"},
	}); err != nil {
		t.Fatal(err)
	}

	for _, mediaID := range []string{"movie:10", "tv:42"} {
		var status string
		if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id='alice' AND media_id=?`, mediaID).Scan(&status); err != nil {
			t.Fatalf("find Alice's %s relationship: %v", mediaID, err)
		}
		if status != "watching" {
			t.Fatalf("Alice's %s status=%q, want watching", mediaID, status)
		}
	}

	// When Bob tracks a paused movie, the play must not reset his chosen status.
	if err := store.CreatePlay(ctx, tracking.Play{
		ID: "bob-movie-play", UserID: "bob", MediaID: &movieID,
		WatchedAt: time.Now().UTC(), Source: "web",
	}); err != nil {
		t.Fatal(err)
	}
	var bobStatus string
	if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id='bob' AND media_id='movie:10'`).Scan(&bobStatus); err != nil {
		t.Fatal(err)
	}
	if bobStatus != "paused" {
		t.Fatalf("Bob's paused state was changed to %q", bobStatus)
	}
}
