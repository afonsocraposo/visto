package sqlite_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestMarkEpisodesThrough_SkipsWatchedEpisodesAndHandlesLargeShows(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example Show',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','watchlist',?,?)`, userID+":tv:42", userID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:0','tv:42',0,'Specials'),('tv:42:season:1','tv:42',1,'Season 1')`); err != nil {
		t.Fatal(err)
	}
	for number := 1; number <= 122; number++ {
		id := fmt.Sprintf("tv:42:episode:%d", number)
		airDate := "2020-01-01"
		if number == 122 {
			airDate = "2100-01-01"
		}
		if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name,air_date) VALUES(?,'tv:42','tv:42:season:1',1,?, 'Episode',?)`, id, number, airDate); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name,air_date) VALUES('tv:42:episode:999','tv:42','tv:42:season:0',0,1,'Special','2020-01-01')`); err != nil {
		t.Fatal(err)
	}
	lastID := "tv:42:episode:121"
	if _, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, EpisodeID: &lastID, WatchedAt: time.Now().UTC(), Source: "mcp"}); err != nil {
		t.Fatal(err)
	}
	service := tracking.NewService(store)
	marked, err := service.MarkEpisodesThrough(ctx, userID, "tv:42", 1, 121, time.Time{}, "mcp")
	if err != nil || marked != 120 {
		t.Fatalf("marked=%d error=%v, want 120", marked, err)
	}
	marked, err = service.MarkEpisodesThrough(ctx, userID, "tv:42", 1, 121, time.Time{}, "mcp")
	if err != nil || marked != 0 {
		t.Fatalf("repeat marked=%d error=%v, want 0", marked, err)
	}
	var count, specialCount int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=?`, userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=? AND episode_id='tv:42:episode:999'`, userID).Scan(&specialCount); err != nil {
		t.Fatal(err)
	}
	if count != 121 || specialCount != 0 {
		t.Fatalf("plays=%d specials=%d, want 121 and 0", count, specialCount)
	}
	var status string
	if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id='tv:42'`, userID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "watching" {
		t.Fatalf("status=%q, want watching", status)
	}
	if _, err := store.DB.Exec(`UPDATE user_media SET status='paused' WHERE user_id=? AND media_id='tv:42'`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkEpisodesThrough(ctx, userID, "tv:42", 1, 121, time.Time{}, "mcp"); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id='tv:42'`, userID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "paused" {
		t.Fatalf("status=%q, want paused", status)
	}
	if _, err := service.MarkEpisodesThrough(ctx, userID, "tv:42", 1, 122, time.Time{}, "mcp"); err == nil {
		t.Fatal("future target should be rejected")
	}
}

func TestMarkSeasonAndSelectedEpisodes_ValidateBeforeWriting(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example Show',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','dropped',?,?)`, userID+":tv:42", userID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season 1'),('tv:42:season:2','tv:42',2,'Season 2')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name,air_date) VALUES
		('tv:42:episode:1','tv:42','tv:42:season:1',1,1,'One','2020-01-01'),
		('tv:42:episode:2','tv:42','tv:42:season:1',1,2,'Two','2020-01-01'),
		('tv:42:episode:3','tv:42','tv:42:season:2',2,1,'Three','2020-01-01')`); err != nil {
		t.Fatal(err)
	}
	service := tracking.NewService(store)
	if _, err := service.MarkSelectedEpisodes(ctx, userID, "tv:42", []string{"tv:42:episode:1", "tv:99:episode:1"}, time.Time{}, "mcp"); err == nil {
		t.Fatal("foreign episode should reject the whole action")
	}
	var count int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=?`, userID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("plays=%d error=%v, want none", count, err)
	}
	marked, err := service.MarkSeasonWatched(ctx, userID, "tv:42", 1, time.Time{}, "mcp")
	if err != nil || marked != 2 {
		t.Fatalf("season marked=%d error=%v, want 2", marked, err)
	}
	marked, err = service.MarkSelectedEpisodes(ctx, userID, "tv:42", []string{"tv:42:episode:2", "tv:42:episode:3"}, time.Time{}, "mcp")
	if err != nil || marked != 1 {
		t.Fatalf("selected marked=%d error=%v, want 1", marked, err)
	}
	var status string
	if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id='tv:42'`, userID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "dropped" {
		t.Fatalf("status=%q, want dropped", status)
	}
}
