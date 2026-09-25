package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/plexsync"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestPlexWebhook_GivenASecretAndRepeatedScrobbles_WhenPersisted_ThenItDeduplicatesAndKeepsBoundedStatus(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES('alice','alice','Alice','hash','user',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_settings(user_id,timezone,activity_visibility,created_at,updated_at) VALUES('alice','UTC','instance',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	if err := store.IssuePlexWebhook(ctx, "alice", "hash-only", created); err != nil {
		t.Fatal(err)
	}
	var storedHash string
	if err := store.DB.QueryRow(`SELECT token_hash FROM plex_webhooks WHERE user_id='alice'`).Scan(&storedHash); err != nil || storedHash != "hash-only" {
		t.Fatalf("stored webhook hash=%q error=%v", storedHash, err)
	}
	userID, err := store.UserForPlexWebhook(ctx, "hash-only", created.Add(time.Minute))
	if err != nil || userID != "alice" {
		t.Fatalf("authenticated user=%q error=%v", userID, err)
	}

	movieID := "movie:10"
	event := plexsync.Event{Status: "synced", Title: "Example Movie", MediaType: "movie", TMDBID: 10, OccurredAt: created}
	recorded, err := store.RecordPlexPlay(ctx, "alice", "event-1", event, &movieID, nil, created, 7*24*time.Hour)
	if err != nil || !recorded {
		t.Fatalf("first event recorded=%v error=%v", recorded, err)
	}
	recorded, err = store.RecordPlexPlay(ctx, "alice", "event-1", event, &movieID, nil, created, 7*24*time.Hour)
	if err != nil || recorded {
		t.Fatalf("exact replay recorded=%v error=%v; want false without error", recorded, err)
	}
	recorded, err = store.RecordPlexPlay(ctx, "alice", "event-2", event, &movieID, nil, created.Add(time.Minute), 7*24*time.Hour)
	if err != nil || recorded {
		t.Fatalf("recent repeat recorded=%v error=%v; want false without error", recorded, err)
	}

	var plays, activityEvents int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id='alice'`).Scan(&plays); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE user_id='alice' AND kind IN ('watch','rewatch')`).Scan(&activityEvents); err != nil {
		t.Fatal(err)
	}
	if plays != 1 || activityEvents != 1 {
		t.Fatalf("plays=%d activity events=%d; want one of each", plays, activityEvents)
	}
	var source string
	if err := store.DB.QueryRow(`SELECT source FROM plays WHERE user_id='alice'`).Scan(&source); err != nil || source != "plex" {
		t.Fatalf("play source=%q error=%v; want plex", source, err)
	}

	// The source column is deliberately open-ended for future integrations.
	if _, err := store.DB.Exec(`UPDATE plays SET source='future-integration' WHERE user_id='alice'`); err != nil {
		t.Fatalf("future source label was rejected by SQLite: %v", err)
	}

	status, err := store.GetPlexWebhookStatus(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.LastUsedAt.IsZero() || status.LastSyncedAt.IsZero() || len(status.RecentEvents) != 2 {
		t.Fatalf("Plex status=%#v; expected active webhook, last use/sync, and two distinct events", status)
	}
	if status.RecentEvents[0].Status != "skipped" || status.RecentEvents[1].Status != "synced" {
		t.Fatalf("recent event statuses=%q,%q; want skipped,synced", status.RecentEvents[0].Status, status.RecentEvents[1].Status)
	}

	if err := store.RevokePlexWebhook(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UserForPlexWebhook(ctx, "hash-only", created.Add(time.Hour)); err == nil {
		t.Fatal("revoked webhook token still authenticated")
	}
}
