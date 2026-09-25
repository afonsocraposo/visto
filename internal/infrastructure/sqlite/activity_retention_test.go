package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestCleanupActivityEvents_GivenOldAndRecentEvents_WhenCleanedInBoundedBatches_ThenItRemovesOnlyOldActivity(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	const userID = "activity-retention-user"
	if _, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(?,?,?,'hash','user',?,?)`, userID, userID, "Retention User", testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:90','movie',90,'Retention Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(id,user_id,media_id,watched_at,source,created_at) VALUES('retained-play',?,'movie:90',?,'web',?)`, userID, "2020-01-01T00:00:00Z", "2020-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO activity_events(id,user_id,kind,play_id,media_id,occurred_at,created_at) VALUES
		('old-event-1',?,'watch','retained-play','movie:90','2020-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
		('old-event-2',?,'watch','retained-play','movie:90','2020-01-02T00:00:00Z','2024-01-02T00:00:00Z'),
		('recent-event',?,'watch','retained-play','movie:90','2020-01-03T00:00:00Z','2026-09-24T00:00:00Z')`, userID, userID, userID); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Date(2025, 9, 25, 0, 0, 0, 0, time.UTC)
	removed, err := store.CleanupActivityEvents(ctx, cutoff, 1)
	if err != nil || removed != 1 {
		t.Fatalf("first cleanup removed=%d err=%v, want one", removed, err)
	}
	removed, err = store.CleanupActivityEvents(ctx, cutoff, 10)
	if err != nil || removed != 1 {
		t.Fatalf("second cleanup removed=%d err=%v, want one", removed, err)
	}
	var events, plays int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE id='retained-play'`).Scan(&plays); err != nil {
		t.Fatal(err)
	}
	if events != 1 || plays != 1 {
		t.Fatalf("events=%d plays=%d, want the recent feed event and original play retained", events, plays)
	}
}
