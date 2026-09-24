package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestLibrary_GivenMovieAndShowPlays_WhenLibraryIsRead_ThenOnlyWatchedMoviesAreCompleted(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES('alice','alice','Alice','hash','user',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES
		('movie:10','movie',10,'Watched Movie',?,?),
		('movie:11','movie',11,'Unwatched Movie',?,?),
		('tv:42','tv',42,'Tracked Show',?,?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season 1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:42:episode:101','tv:42','tv:42:season:1',1,1,'Pilot')`); err != nil {
		t.Fatal(err)
	}
	for _, mediaID := range []string{"movie:10", "movie:11", "tv:42"} {
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?, 'alice', ?, 'watching', ?, ?)`, "alice:"+mediaID, mediaID, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(id,user_id,media_id,watched_at,source,created_at) VALUES('movie-play','alice','movie:10',?,'web',?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(id,user_id,episode_id,watched_at,source,created_at) VALUES('episode-play','alice','tv:42:episode:101',?,'web',?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}

	// When the library is read, completion is derived from movie plays only.
	entries, err := store.ListItems(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	completed := map[string]bool{}
	for _, entry := range entries {
		completed[entry.Item.MediaID] = entry.Completed
	}
	if !completed["movie:10"] || completed["movie:11"] || completed["tv:42"] {
		t.Fatalf("derived completion states=%v", completed)
	}
	movie, err := store.GetMediaByTMDBID(ctx, "alice", domain.MovieMediaType, 10)
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
