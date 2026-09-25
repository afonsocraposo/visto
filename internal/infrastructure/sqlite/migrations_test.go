package sqlite_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	_ "modernc.org/sqlite"
)

func TestMigrator_GivenFreshDatabase_WhenAppliedTwice_ThenSchemaIsCreatedAndSecondRunIsANoOp(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	migrator.Now = func() time.Time { return time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC) }

	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != len(migrator.Migrations) {
		t.Fatalf("migration count = %d, want %d", count, len(migrator.Migrations))
	}
	for _, table := range []string{"users", "media", "episodes", "plays", "activity_events", "episode_ratings", "personal_api_tokens", "notification_deliveries", "oauth_clients", "oauth_authorization_codes", "oauth_access_tokens", "oauth_refresh_tokens", "plex_webhooks", "plex_webhook_events"} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %q: %v", table, err)
		}
	}
	var column string
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('media') WHERE name='catalog_updated_at'`).Scan(&column); err != nil {
		t.Fatalf("expected catalog refresh timestamp migration: %v", err)
	}
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('oauth_access_tokens') WHERE name='last_used_at'`).Scan(&column); err != nil {
		t.Fatalf("expected OAuth connection metadata migration: %v", err)
	}
}

func TestMigrator_GivenExistingPlaysAndFeedActivity_WhenPlexSourceIsAdded_ThenMigrationPreservesBoth(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatal(err)
	}
	migrations := migrator.Migrations
	migrator.Migrations = migrations[:10]
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("apply original schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES('alice','alice','Alice','hash','user',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO plays(id,user_id,media_id,watched_at,source,created_at) VALUES('play-1','alice','movie:10',?,'web',?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO activity_events(id,user_id,kind,play_id,media_id,occurred_at,created_at) VALUES('activity-1','alice','watch','play-1','movie:10',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	migrator.Migrations = migrations
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("apply Plex source migration: %v", err)
	}
	var source, playID string
	if err := db.QueryRow(`SELECT p.source,a.play_id FROM plays p JOIN activity_events a ON a.play_id=p.id WHERE p.id='play-1'`).Scan(&source, &playID); err != nil {
		t.Fatalf("existing play/feed row was not preserved: %v", err)
	}
	if source != "web" || playID != "play-1" {
		t.Fatalf("preserved source=%q play ID=%q", source, playID)
	}
}

func TestMigrator_GivenChangedAppliedMigration_WhenApplied_ThenItFails(t *testing.T) {
	db := openTestDatabase(t)
	first := &sqlite.Migrator{Migrations: []sqlite.Migration{{Version: 1, Name: "create_items", SQL: "CREATE TABLE items (id INTEGER);", Checksum: "first"}}, Now: time.Now}
	if err := first.Apply(context.Background(), db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	changed := &sqlite.Migrator{Migrations: []sqlite.Migration{{Version: 1, Name: "create_items", SQL: "CREATE TABLE items (id INTEGER, name TEXT);", Checksum: "changed"}}, Now: time.Now}

	err := changed.Apply(context.Background(), db)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want checksum mismatch", err)
	}
}

func TestLoadMigrations_GivenInvalidFilename_WhenLoaded_ThenItFails(t *testing.T) {
	_, err := sqlite.LoadMigrations(fstest.MapFS{"migrations/not-a-migration.txt": {Data: []byte("SELECT 1;")}})
	if err == nil || !strings.Contains(err.Error(), "invalid migration filename") {
		t.Fatalf("error = %v, want invalid filename error", err)
	}
}

func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
