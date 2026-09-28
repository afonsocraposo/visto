package sqlite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/importer"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestSuppliedBingersArchive(t *testing.T) {
	archivePath := os.Getenv("VISTO_BINGERS_TEST_ZIP")
	if archivePath == "" {
		t.Skip("set VISTO_BINGERS_TEST_ZIP to verify a private export")
	}
	payload, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "archive-check", "Archive Check", "private")
	result, err := importer.NewService(store, nil).Import(context.Background(), userID, "bingers", payload)
	if err != nil {
		t.Fatal(err)
	}
	if result.Titles != 75 || result.Watches != 3849 || result.Skipped != 20 {
		t.Fatalf("import counts: titles=%d watches=%d skipped=%d", result.Titles, result.Watches, result.Skipped)
	}
	var shows, episodes int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM media WHERE media_type='tv' AND metadata_updated_at=''`).Scan(&shows); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM episodes WHERE metadata_updated_at IS NULL`).Scan(&episodes); err != nil {
		t.Fatal(err)
	}
	if shows != 75 || episodes != 3849 {
		t.Fatalf("isolated backlog: shows=%d episodes=%d", shows, episodes)
	}
}

func TestImportDataMergesWithoutDuplicatingOrPublishing(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	alice := insertTestUser(t, store.DB, "import-alice", "Alice", "instance")
	bob := insertTestUser(t, store.DB, "import-bob", "Bob", "instance")
	pending, err := store.ImportWelcomePending(ctx, bob)
	if err != nil || !pending {
		t.Fatalf("new account welcome pending=%t, error=%v", pending, err)
	}
	if err := store.DismissImportWelcome(ctx, bob); err != nil {
		t.Fatal(err)
	}
	pending, err = store.ImportWelcomePending(ctx, bob)
	if err != nil || pending {
		t.Fatalf("dismissed welcome pending=%t, error=%v", pending, err)
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Existing','','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at) VALUES(?,?, 'tv:42','dropped',5,?,?)`, alice+":tv:42", alice, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	data := importer.Data{
		Titles:  []importer.Title{{Type: "tv", Name: "Example", TMDBID: 42, Status: "watching", AddedAt: when}, {Type: "movie", Name: "Film", TMDBID: 10, Status: "watchlist", AddedAt: when}},
		Watches: []importer.Watch{{Type: "episode", Name: "Example", TMDBID: 42, Season: 1, Episode: 1, WatchedAt: when}, {Type: "movie", Name: "Film", TMDBID: 10, WatchedAt: when}},
		Ratings: []importer.Rating{{Type: "tv", Name: "Example", TMDBID: 42, Value: 3}, {Type: "episode", Name: "Example", TMDBID: 42, Season: 1, Episode: 1, Value: 4}},
	}
	first, err := store.ImportData(ctx, alice, data)
	if err != nil {
		t.Fatal(err)
	}
	if first.Titles != 1 || first.Watches != 2 || first.Ratings != 1 {
		t.Fatalf("first import = %+v", first)
	}
	second, err := store.ImportData(ctx, alice, data)
	if err != nil {
		t.Fatal(err)
	}
	if second.Titles != 0 || second.Watches != 0 || second.Ratings != 0 {
		t.Fatalf("repeat import = %+v", second)
	}
	var status string
	var rating int
	if err := store.DB.QueryRow(`SELECT status,rating FROM user_media WHERE user_id=? AND media_id='tv:42'`, alice).Scan(&status, &rating); err != nil {
		t.Fatal(err)
	}
	if status != "dropped" || rating != 5 {
		t.Fatalf("existing choices changed: %s, %d", status, rating)
	}
	if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id='movie:10'`, alice).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("movie status=%q, error=%v", status, err)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM plays WHERE user_id=?`,
		`SELECT COUNT(*) FROM activity_events WHERE user_id=?`,
		`SELECT COUNT(*) FROM user_media WHERE user_id=?`,
	} {
		var count int
		if err := store.DB.QueryRow(query, bob).Scan(&count); err != nil || count != 0 {
			t.Fatalf("other account count=%d, error=%v", count, err)
		}
	}
	var events int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE user_id=?`, alice).Scan(&events); err != nil || events != 0 {
		t.Fatalf("import activity events=%d, error=%v", events, err)
	}
	pending, err = store.ImportWelcomePending(ctx, alice)
	if err != nil || pending {
		t.Fatalf("welcome pending=%t, error=%v", pending, err)
	}
}

func TestImportCompletesFullyWatchedEndedShowWithExistingCatalog(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "complete-import", "Complete Import", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,status,metadata_updated_at,created_at)
		VALUES('tv:1920','tv',1920,'Twin Peaks','Ended',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name,episode_count)
		VALUES('tv:1920:season:1','tv:1920',1,'Season 1',2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name)
		VALUES('e1','tv:1920','tv:1920:season:1',1,1,'One'),('e2','tv:1920','tv:1920:season:1',1,2,'Two')`); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	data := importer.Data{
		Titles: []importer.Title{{Type: "tv", Name: "Twin Peaks", TMDBID: 1920, Status: "watching", AddedAt: when}},
		Watches: []importer.Watch{
			{Type: "episode", Name: "Twin Peaks", TMDBID: 1920, Season: 1, Episode: 1, WatchedAt: when},
			{Type: "episode", Name: "Twin Peaks", TMDBID: 1920, Season: 1, Episode: 2, WatchedAt: when},
		},
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := store.ImportData(ctx, userID, data); err != nil {
			t.Fatal(err)
		}
		var status string
		if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id='tv:1920'`, userID).Scan(&status); err != nil || status != "completed" {
			t.Fatalf("attempt %d status=%q err=%v", attempt, status, err)
		}
	}
}
