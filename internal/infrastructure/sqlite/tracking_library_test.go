package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestRemoveWatchlistItem_OnlyRemovesUsersWatchlistEntry(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	aliceID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	bobID := insertTestUser(t, store.DB, "bob", "Bob", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ userID, status string }{{aliceID, "watchlist"}, {bobID, "watchlist"}} {
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?, 'movie:10',?,?,?)`, item.userID+":movie:10", item.userID, item.status, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RemoveWatchlistItem(ctx, aliceID, "movie:10"); err != nil {
		t.Fatal(err)
	}
	var aliceCount, bobCount int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM user_media WHERE user_id=?`, aliceID).Scan(&aliceCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM user_media WHERE user_id=?`, bobID).Scan(&bobCount); err != nil {
		t.Fatal(err)
	}
	if aliceCount != 0 || bobCount != 1 {
		t.Fatalf("library entries Alice=%d Bob=%d, want 0 and 1", aliceCount, bobCount)
	}
	if _, err := store.DB.Exec(`UPDATE user_media SET status='watching' WHERE user_id=?`, bobID); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveWatchlistItem(ctx, bobID, "movie:10"); err == nil {
		t.Fatal("watching item should not be removed as a watchlist item")
	}
}

func TestMovieWatch_MovesMovieOutOfWatchlist(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'movie:10','watchlist',?,?)`, userID+":movie:10", userID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	mediaID := "movie:10"
	if _, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, MediaID: &mediaID, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id=?`, userID, mediaID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "watching" {
		t.Fatalf("movie status=%q, want watching", status)
	}
}

func TestRemoveWatchingItem_KeepsTitlesWithWatchHistory(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Movie',?,?),('movie:11','movie',11,'Other movie',?,?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	for _, mediaID := range []string{"movie:10", "movie:11"} {
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,?,'watching',?,?)`, userID+":"+mediaID, userID, mediaID, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	playedID := "movie:11"
	if _, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, MediaID: &playedID, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveWatchingItem(ctx, userID, "movie:10"); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveWatchingItem(ctx, userID, playedID); err == nil {
		t.Fatal("watching entry with a play should remain")
	}
	var remaining int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM user_media WHERE user_id=?`, userID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("remaining entries=%d, want 1", remaining)
	}
}

func TestRemoveWatchingShow_PreservesEpisodeHistory(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Show',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season 1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:42:episode:1','tv:42','tv:42:season:1',1,1,'Pilot')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','watching',?,?)`, userID+":tv:42", userID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	episodeID := "tv:42:episode:1"
	if _, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, EpisodeID: &episodeID, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveWatchingItem(ctx, userID, "tv:42"); err != nil {
		t.Fatal(err)
	}
	var libraryCount, playCount int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM user_media WHERE user_id=? AND media_id='tv:42'`, userID).Scan(&libraryCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=? AND episode_id=?`, userID, episodeID).Scan(&playCount); err != nil {
		t.Fatal(err)
	}
	if libraryCount != 0 || playCount != 1 {
		t.Fatalf("library entries=%d, plays=%d; want 0 and 1", libraryCount, playCount)
	}
}

func TestRemoveStatusItem_OnlyRemovesMatchingStatus(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Show',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','paused',?,?)`, userID+":tv:42", userID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveStatusItem(ctx, userID, "tv:42", domain.DroppedStatus); err == nil {
		t.Fatal("wrong status removed the item")
	}
	if err := store.RemoveStatusItem(ctx, userID, "tv:42", domain.PausedStatus); err != nil {
		t.Fatal(err)
	}
}

func TestListMediaPlays_FiltersBeforeApplyingLimit(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'First',?,?),('movie:11','movie',11,'Second',?,?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	for _, mediaID := range []string{"movie:10", "movie:11"} {
		if _, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, MediaID: &mediaID, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := store.ListMediaPlays(ctx, userID, "movie:10", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Play.MediaID == nil || *entries[0].Play.MediaID != "movie:10" {
		t.Fatalf("filtered history=%+v, want one movie:10 watch", entries)
	}
}

func TestTracking_GivenNewAndExistingTitles_WhenPlaysAreRecorded_ThenItCreatesOrPreservesLibraryRelationships(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	aliceID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	bobID := insertTestUser(t, store.DB, "bob", "Bob", "private")
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
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'movie:10','paused',?,?)`, bobID+":movie:10", bobID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}

	// When Alice records a movie play, create the missing relationship as watching.
	movieID := "movie:10"
	if _, err := store.CreatePlay(ctx, tracking.Play{
		UserID: aliceID, MediaID: &movieID,
		WatchedAt: time.Now().UTC(), Source: "web",
	}); err != nil {
		t.Fatal(err)
	}
	// A bulk episode action also creates the show relationship.
	episodeIDs := []string{"tv:42:episode:101", "tv:42:episode:102"}
	if _, err := store.CreateBulkPlays(ctx, []tracking.Play{
		{UserID: aliceID, EpisodeID: &episodeIDs[0], WatchedAt: time.Now().UTC(), Source: "web"},
		{UserID: aliceID, EpisodeID: &episodeIDs[1], WatchedAt: time.Now().UTC(), Source: "web"},
	}); err != nil {
		t.Fatal(err)
	}

	for _, mediaID := range []string{"movie:10", "tv:42"} {
		var status string
		if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id=?`, aliceID, mediaID).Scan(&status); err != nil {
			t.Fatalf("find Alice's %s relationship: %v", mediaID, err)
		}
		if status != "watching" {
			t.Fatalf("Alice's %s status=%q, want watching", mediaID, status)
		}
	}

	// When Bob tracks a paused movie, the play must not reset his chosen status.
	if _, err := store.CreatePlay(ctx, tracking.Play{
		UserID: bobID, MediaID: &movieID,
		WatchedAt: time.Now().UTC(), Source: "web",
	}); err != nil {
		t.Fatal(err)
	}
	var bobStatus string
	if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id='movie:10'`, bobID).Scan(&bobStatus); err != nil {
		t.Fatal(err)
	}
	if bobStatus != "paused" {
		t.Fatalf("Bob's paused state was changed to %q", bobStatus)
	}
}
