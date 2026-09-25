package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestLibrary_GivenMovieAndShowPlays_WhenLibraryIsRead_ThenCompletionAndRegularEpisodeProgressAreDerived(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	aliceID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	bobID := insertTestUser(t, store.DB, "bob", "Bob", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES
		('movie:10','movie',10,'Watched Movie',?,?),
		('movie:11','movie',11,'Unwatched Movie',?,?),
		('tv:42','tv',42,'Tracked Show',?,?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name,episode_count) VALUES
		('tv:42:season:0','tv:42',0,'Specials',1),('tv:42:season:1','tv:42',1,'Season 1',2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES
		('tv:42:episode:100','tv:42','tv:42:season:0',0,1,'Special'),
		('tv:42:episode:101','tv:42','tv:42:season:1',1,1,'Pilot'),
		('tv:42:episode:102','tv:42','tv:42:season:1',1,2,'Second Episode')`); err != nil {
		t.Fatal(err)
	}
	for _, mediaID := range []string{"movie:10", "movie:11", "tv:42"} {
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?, ?, ?, 'watching', ?, ?)`, aliceID+":"+mediaID, aliceID, mediaID, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','watching',?,?)`, bobID+":tv:42", bobID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(user_id,media_id,watched_at,source,created_at) VALUES(?,'movie:10',?,'web',?)`, aliceID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES
		(?,'tv:42:episode:101',?,'web',?),
		(?,'tv:42:episode:100',?,'web',?)`, aliceID, testTimestamp, testTimestamp, aliceID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}

	// When the library is read, show progress counts watched regular episodes but excludes specials.
	entries, err := store.ListItems(ctx, aliceID)
	if err != nil {
		t.Fatal(err)
	}
	completed := map[string]bool{}
	progress := map[string]*struct{ watched, total int }{}
	for _, entry := range entries {
		completed[entry.Item.MediaID] = entry.Completed
		if entry.Progress != nil {
			progress[entry.Item.MediaID] = &struct{ watched, total int }{entry.Progress.WatchedEpisodes, entry.Progress.TotalEpisodes}
		}
	}
	if !completed["movie:10"] || completed["movie:11"] || completed["tv:42"] {
		t.Fatalf("derived completion states=%v", completed)
	}
	if got := progress["tv:42"]; got == nil || got.watched != 1 || got.total != 2 {
		t.Fatalf("show progress=%+v, want 1 of 2 regular episodes (specials excluded)", got)
	}
	if progress["movie:10"] != nil || progress["movie:11"] != nil {
		t.Fatalf("movie progress should not be present: %+v", progress)
	}
	bobEntries, err := store.ListItems(ctx, bobID)
	if err != nil {
		t.Fatal(err)
	}
	if got := bobEntries[0].Progress; got == nil || got.WatchedEpisodes != 0 || got.TotalEpisodes != 2 {
		t.Fatalf("Bob's show progress=%+v, want 0 of 2 despite Alice's plays", got)
	}
	movie, err := store.GetMediaByTMDBID(ctx, aliceID, domain.MovieMediaType, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !movie.Completed {
		t.Fatalf("movie detail did not derive completion: %+v", movie)
	}
	var storedCompletedStatus int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM user_media WHERE status='completed'`).Scan(&storedCompletedStatus); err != nil {
		t.Fatal(err)
	}
	if storedCompletedStatus != 0 {
		t.Fatal("computed completion must not be stored as a library status")
	}
}
