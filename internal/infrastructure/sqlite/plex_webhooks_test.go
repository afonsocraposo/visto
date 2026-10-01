package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/plexsync"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestSharedPlexWebhook_MapsObservedAccountsAndPreservesPersonalURLs(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	alice := insertTestUser(t, store.DB, "alice", "Alice", "private")
	bob := insertTestUser(t, store.DB, "bob", "Bob", "private")
	now := time.Now().UTC()
	if err := store.IssuePlexWebhook(ctx, alice, "personal-hash", "123", now); err != nil {
		t.Fatal(err)
	}
	if err := store.IssueSharedPlexWebhook(ctx, "shared-hash", now); err != nil {
		t.Fatal(err)
	}
	if valid, err := store.SharedPlexWebhookByToken(ctx, "shared-hash", now); err != nil || !valid {
		t.Fatalf("shared token valid=%v err=%v", valid, err)
	}
	if userID, accountID, err := store.UserForPlexWebhook(ctx, "personal-hash", now); err != nil || userID != alice || accountID != "123" {
		t.Fatalf("personal token user=%s account=%s err=%v", userID, accountID, err)
	}
	if err := store.ObservePlexAccount(ctx, "456", "Family", now); err != nil {
		t.Fatal(err)
	}
	accounts, err := store.ListPlexAccounts(ctx)
	if err != nil || len(accounts) != 1 || accounts[0].Title != "Family" {
		t.Fatalf("observed accounts=%#v err=%v", accounts, err)
	}
	if _, err := store.PlexUserForAccount(ctx, "456"); !errors.Is(err, plexsync.ErrAccountUnmapped) {
		t.Fatalf("unmapped account error=%v", err)
	}
	if err := store.SetPlexMapping(ctx, bob, "456"); err != nil {
		t.Fatal(err)
	}
	if userID, err := store.PlexUserForAccount(ctx, "456"); err != nil || userID != bob {
		t.Fatalf("mapped user=%s err=%v", userID, err)
	}
	if err := store.SetPlexMapping(ctx, alice, "456"); err == nil {
		t.Fatal("duplicate Plex account mapping accepted")
	}
	if err := store.RevokeSharedPlexWebhook(ctx); err != nil {
		t.Fatal(err)
	}
	if valid, err := store.SharedPlexWebhookByToken(ctx, "shared-hash", now); err != nil || valid {
		t.Fatalf("revoked shared token valid=%v err=%v", valid, err)
	}
	for i := 0; i < 101; i++ {
		if err := store.ObservePlexAccount(ctx, fmt.Sprint(1000+i), "Family", now.Add(time.Duration(i+1)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	accounts, err = store.ListPlexAccounts(ctx)
	if err != nil || len(accounts) != 100 {
		t.Fatalf("retained accounts=%d err=%v; want 100", len(accounts), err)
	}
}

func TestPlexWebhook_GivenASecretAndRepeatedScrobbles_WhenPersisted_ThenItDeduplicatesAndKeepsBoundedStatus(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "alice", "Alice", "instance")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC()
	if err := store.IssuePlexWebhook(ctx, userID, "hash-only", "123", created); err != nil {
		t.Fatal(err)
	}
	var storedHash string
	if err := store.DB.QueryRow(`SELECT token_hash FROM plex_webhooks WHERE user_id=?`, userID).Scan(&storedHash); err != nil || storedHash != "hash-only" {
		t.Fatalf("stored webhook hash=%q error=%v", storedHash, err)
	}
	authenticatedUserID, accountID, err := store.UserForPlexWebhook(ctx, "hash-only", created.Add(time.Minute))
	if err != nil || authenticatedUserID != userID || accountID != "123" {
		t.Fatalf("authenticated user=%q error=%v", authenticatedUserID, err)
	}

	movieID := "movie:10"
	event := plexsync.Event{Status: "synced", Title: "Example Movie", MediaType: "movie", TMDBID: 10, OccurredAt: created}
	recorded, err := store.RecordPlexPlay(ctx, userID, "event-1", event, &movieID, nil, created, 7*24*time.Hour)
	if err != nil || !recorded {
		t.Fatalf("first event recorded=%v error=%v", recorded, err)
	}
	recorded, err = store.RecordPlexPlay(ctx, userID, "event-1", event, &movieID, nil, created, 7*24*time.Hour)
	if err != nil || recorded {
		t.Fatalf("exact replay recorded=%v error=%v; want false without error", recorded, err)
	}
	recorded, err = store.RecordPlexPlay(ctx, userID, "event-2", event, &movieID, nil, created.Add(time.Minute), 7*24*time.Hour)
	if err != nil || recorded {
		t.Fatalf("recent repeat recorded=%v error=%v; want false without error", recorded, err)
	}

	var plays, activityEvents int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=?`, userID).Scan(&plays); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE user_id=? AND kind IN ('watch','rewatch')`, userID).Scan(&activityEvents); err != nil {
		t.Fatal(err)
	}
	if plays != 1 || activityEvents != 1 {
		t.Fatalf("plays=%d activity events=%d; want one of each", plays, activityEvents)
	}
	var source string
	if err := store.DB.QueryRow(`SELECT source FROM plays WHERE user_id=?`, userID).Scan(&source); err != nil || source != "plex" {
		t.Fatalf("play source=%q error=%v; want plex", source, err)
	}

	// The source column is deliberately open-ended for future integrations.
	if _, err := store.DB.Exec(`UPDATE plays SET source='future-integration' WHERE user_id=?`, userID); err != nil {
		t.Fatalf("future source label was rejected by SQLite: %v", err)
	}

	status, err := store.GetPlexWebhookStatus(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.LastUsedAt.IsZero() || status.LastSyncedAt.IsZero() || len(status.RecentEvents) != 2 {
		t.Fatalf("Plex status=%#v; expected active webhook, last use/sync, and two distinct events", status)
	}
	if status.RecentEvents[0].Status != "skipped" || status.RecentEvents[1].Status != "synced" {
		t.Fatalf("recent event statuses=%q,%q; want skipped,synced", status.RecentEvents[0].Status, status.RecentEvents[1].Status)
	}

	if err := store.RevokePlexWebhook(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.UserForPlexWebhook(ctx, "hash-only", created.Add(time.Hour)); err == nil {
		t.Fatal("revoked webhook token still authenticated")
	}
}

func TestPlexEventPayloads_GivenStoredEvents_WhenReadBack_ThenPayloadsAreScopedTrimmedAndOptional(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	alice := insertTestUser(t, store.DB, "alice", "Alice", "instance")
	bob := insertTestUser(t, store.DB, "bob", "Bob", "instance")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	skipped := plexsync.Event{Status: "skipped", EventType: "media.stop", RawPayload: `{"event":"media.stop"}`, OccurredAt: now}
	if err := store.LogPlexEvent(ctx, alice, "skip", skipped); err != nil {
		t.Fatal(err)
	}
	movieID := "movie:10"
	synced := plexsync.Event{Status: "synced", EventType: "media.scrobble", RawPayload: `{"event":"media.scrobble"}`, OccurredAt: now}
	if recorded, err := store.RecordPlexPlay(ctx, alice, "sync", synced, &movieID, nil, now, time.Hour); err != nil || !recorded {
		t.Fatalf("recorded=%v err=%v", recorded, err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plex_webhook_events(user_id,fingerprint,status,occurred_at,created_at) VALUES(?,'legacy','skipped',?,?)`, alice, now.Format(time.RFC3339Nano), now.Add(-time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	status, err := store.GetPlexWebhookStatus(ctx, alice)
	if err != nil || len(status.RecentEvents) != 3 {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	byType := map[string]int64{}
	var legacyID int64
	for _, event := range status.RecentEvents {
		if event.EventType == "" {
			legacyID = event.ID
		}
		byType[event.EventType] = event.ID
	}
	if payload, err := store.GetPlexEventPayload(ctx, alice, byType["media.stop"]); err != nil || payload != skipped.RawPayload {
		t.Fatalf("skipped payload=%q err=%v", payload, err)
	}
	if payload, err := store.GetPlexEventPayload(ctx, alice, byType["media.scrobble"]); err != nil || payload != synced.RawPayload {
		t.Fatalf("synced payload=%q err=%v", payload, err)
	}
	if payload, err := store.GetPlexEventPayload(ctx, alice, legacyID); err != nil || payload != "" {
		t.Fatalf("legacy payload=%q err=%v; want empty", payload, err)
	}
	if _, err := store.GetPlexEventPayload(ctx, bob, byType["media.stop"]); !errors.Is(err, plexsync.ErrEventNotFound) {
		t.Fatalf("other user's payload err=%v; want not found", err)
	}
	for i := 0; i < 110; i++ {
		event := plexsync.Event{Status: "skipped", EventType: "media.play", RawPayload: "{}", OccurredAt: now}
		if err := store.LogPlexEvent(ctx, alice, fmt.Sprintf("bulk-%d", i), event); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plex_webhook_events WHERE user_id=?`, alice).Scan(&count); err != nil || count != 100 {
		t.Fatalf("event count=%d err=%v; want 100", count, err)
	}
}

func TestPlexMatching_FindsCataloguedEpisodesByNumberAndByTMDBID(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:100','tv',100,'The Paper','` + now + `','` + now + `')`,
		`INSERT INTO seasons(id,show_id,season_number) VALUES('tv:100:season:2','tv:100',2)`,
		`INSERT INTO episodes(id,show_id,season_id,tmdb_id,season_number,episode_number,active) VALUES('tv:100:episode:7423219','tv:100','tv:100:season:2',7423219,2,4,1)`,
		`INSERT INTO episodes(id,show_id,season_id,tmdb_id,season_number,episode_number,active) VALUES('tv:100:episode:9005','tv:100','tv:100:season:2',9005,2,5,0)`,
		`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,active) VALUES('tv:100:episode:imported','tv:100','tv:100:season:2',2,6,1)`,
	} {
		if _, err := store.DB.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	want := plexsync.LocalEpisode{EpisodeID: "tv:100:episode:7423219", ShowTMDBID: 100, ShowTitle: "The Paper", Season: 2, Episode: 4}
	if found, ok, err := store.LocalEpisode(ctx, 100, 2, 4); err != nil || !ok || found != want {
		t.Fatalf("by number = %#v, %v, %v; want %#v", found, ok, err, want)
	}
	if found, ok, err := store.LocalEpisodeByTMDBID(ctx, 7423219); err != nil || !ok || found != want {
		t.Fatalf("by TMDB ID = %#v, %v, %v; want the episode and its show %#v", found, ok, err, want)
	}
	if _, ok, err := store.LocalEpisodeByTMDBID(ctx, 9005); err != nil || ok {
		t.Fatalf("inactive episode by TMDB ID found=%v err=%v; want ignored", ok, err)
	}
	if _, ok, err := store.LocalEpisodeByTMDBID(ctx, 1); err != nil || ok {
		t.Fatalf("unknown TMDB ID found=%v err=%v; want not found", ok, err)
	}
	if _, ok, err := store.LocalEpisodeByTMDBID(ctx, 0); err != nil || ok {
		t.Fatalf("zero TMDB ID found=%v err=%v; want not found", ok, err)
	}
	if _, ok, err := store.LocalEpisode(ctx, 100, 2, 5); err != nil || ok {
		t.Fatalf("inactive episode by number found=%v err=%v; want ignored", ok, err)
	}
	var indexed int
	if err := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_episodes_tmdb_id'`).Scan(&indexed); err != nil || indexed != 1 {
		t.Fatalf("episode TMDB ID index present=%d err=%v; want the lookup indexed", indexed, err)
	}
}

func TestPlexEvents_StoreAndListTheEpisodeLabel(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user := insertTestUser(t, store.DB, "alice", "Alice", "private")
	event := plexsync.Event{Status: "skipped", Title: "The Paper", EpisodeLabel: "S2E4", MediaType: "episode", EventType: "media.scrobble", Message: "no match", OccurredAt: time.Now().UTC()}
	if err := store.LogPlexEvent(ctx, user, "fingerprint-1", event); err != nil {
		t.Fatal(err)
	}
	movie := plexsync.Event{Status: "skipped", Title: "Dune", MediaType: "movie", EventType: "media.scrobble", OccurredAt: time.Now().UTC()}
	if err := store.LogPlexEvent(ctx, user, "fingerprint-2", movie); err != nil {
		t.Fatal(err)
	}
	status, err := store.GetPlexWebhookStatus(ctx, user)
	if err != nil || len(status.RecentEvents) != 2 {
		t.Fatalf("status=%#v err=%v; want two events", status, err)
	}
	byTitle := map[string]plexsync.Event{}
	for _, listed := range status.RecentEvents {
		byTitle[listed.Title] = listed
	}
	if byTitle["The Paper"].EpisodeLabel != "S2E4" || byTitle["Dune"].EpisodeLabel != "" {
		t.Fatalf("listed events = %#v, want S2E4 on the episode only", byTitle)
	}
}

// Events logged before episode labels existed are filled in from their stored payload.
func TestMigration_BackfillsEpisodeLabelsFromStoredPayloads(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user := insertTestUser(t, store.DB, "alice", "Alice", "private")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	episodePayload := `{"event":"media.scrobble","Metadata":{"type":"episode","title":"Impressing a Cop","grandparentTitle":"The Paper (2025)","parentIndex":2,"index":4}}`
	moviePayload := `{"event":"media.scrobble","Metadata":{"type":"movie","title":"Dune"}}`
	rows := []struct{ fingerprint, status, title, payload string }{
		{"a", "skipped", "Impressing a Cop", episodePayload},
		{"b", "synced", "The Paper", episodePayload},
		{"c", "skipped", "Dune", moviePayload},
		{"d", "failed", "", "not json"},
		{"e", "skipped", "Old event", ""},
	}
	for _, row := range rows {
		payload := any(row.payload)
		if row.payload == "" {
			payload = nil
		}
		if _, err := store.DB.ExecContext(ctx, `INSERT INTO plex_webhook_events(user_id,fingerprint,status,title,occurred_at,created_at,raw_payload) VALUES(?,?,?,?,?,?,?)`,
			user, row.fingerprint, row.status, row.title, now, now, payload); err != nil {
			t.Fatal(err)
		}
	}
	script, err := os.ReadFile("migrations/0017_plex_episode_lookup_and_labels.sql")
	if err != nil {
		t.Fatal(err)
	}
	backfill := string(script[strings.Index(string(script), "UPDATE plex_webhook_events"):])
	if _, err := store.DB.ExecContext(ctx, backfill); err != nil {
		t.Fatal(err)
	}
	got := map[string][2]string{}
	result, err := store.DB.QueryContext(ctx, `SELECT fingerprint,COALESCE(title,''),COALESCE(episode_label,'') FROM plex_webhook_events`)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	for result.Next() {
		var fingerprint, title, label string
		if err := result.Scan(&fingerprint, &title, &label); err != nil {
			t.Fatal(err)
		}
		got[fingerprint] = [2]string{title, label}
	}
	want := map[string][2]string{
		"a": {"The Paper (2025)", "S2E4"}, // skipped: named after the show, not the episode
		"b": {"The Paper", "S2E4"},        // synced: keeps the TMDB show name
		"c": {"Dune", ""},
		"d": {"", ""},
		"e": {"Old event", ""},
	}
	for fingerprint, expected := range want {
		if got[fingerprint] != expected {
			t.Errorf("event %s = %v, want %v", fingerprint, got[fingerprint], expected)
		}
	}
}
