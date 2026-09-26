package sqlite_test

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	_ "modernc.org/sqlite"
)

func TestMigrator_GivenFreshDatabase_WhenAppliedTwice_ThenCurrentSchemaExistsAndSecondRunIsNoOp(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	if len(migrator.Migrations) != 2 || migrator.Migrations[0].Version != 1 || migrator.Migrations[1].Version != 2 {
		t.Fatalf("loaded migrations = %#v, want baseline and Plex account migrations", migrator.Migrations)
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
	if count != 2 {
		t.Fatalf("migration count = %d, want 2", count)
	}
	var version int
	var name string
	if err := db.QueryRow(`SELECT version, name FROM schema_migrations WHERE version=1`).Scan(&version, &name); err != nil {
		t.Fatalf("read baseline migration: %v", err)
	}
	if version != 1 || name != "initial_schema" {
		t.Fatalf("applied migration = (%d, %q), want (1, initial_schema)", version, name)
	}

	for _, table := range []string{
		"users", "user_settings", "sessions", "media", "seasons", "episodes", "user_media", "plays",
		"activity_events", "episode_ratings", "personal_api_tokens", "notification_deliveries", "oauth_clients",
		"oauth_authorization_codes", "oauth_access_tokens", "oauth_refresh_tokens", "plex_webhooks", "plex_webhook_events",
	} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %q: %v", table, err)
		}
	}

	for _, table := range []string{"users", "sessions", "plays", "activity_events", "personal_api_tokens"} {
		var columnType string
		var primaryKey int
		if err := db.QueryRow(`SELECT type, pk FROM pragma_table_info(?) WHERE name = 'id'`, table).Scan(&columnType, &primaryKey); err != nil {
			t.Fatalf("read %s.id schema: %v", table, err)
		}
		if !strings.EqualFold(columnType, "INTEGER") || primaryKey != 1 {
			t.Errorf("%s.id = (%s, pk=%d), want INTEGER primary key", table, columnType, primaryKey)
		}
	}
	for _, table := range []string{
		"user_settings", "sessions", "user_media", "plays", "activity_events", "episode_ratings",
		"personal_api_tokens", "notification_deliveries", "oauth_authorization_codes", "oauth_access_tokens",
		"oauth_refresh_tokens", "plex_webhooks", "plex_webhook_events",
	} {
		var columnType string
		if err := db.QueryRow(`SELECT type FROM pragma_table_info(?) WHERE name = 'user_id'`, table).Scan(&columnType); err != nil {
			t.Fatalf("read %s.user_id schema: %v", table, err)
		}
		if !strings.EqualFold(columnType, "INTEGER") {
			t.Errorf("%s.user_id has type %s, want INTEGER", table, columnType)
		}
	}

	for _, check := range []struct{ table, column string }{
		{"media", "catalog_updated_at"},
		{"oauth_access_tokens", "last_used_at"},
		{"activity_events", "created_at"},
	} {
		var column string
		if err := db.QueryRow(`SELECT name FROM pragma_table_info(?) WHERE name = ?`, check.table, check.column).Scan(&column); err != nil {
			t.Errorf("expected %s.%s: %v", check.table, check.column, err)
		}
	}
	var index string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_activity_events_created_at'`).Scan(&index); err != nil {
		t.Fatalf("expected activity-event retention index: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO users(username,display_name,password_hash,role,created_at,updated_at) VALUES('alice','Alice','hash','user','now','now')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie','now','now')`); err != nil {
		t.Fatalf("insert media: %v", err)
	}
	var userID int64
	if err := db.QueryRow(`SELECT id FROM users WHERE username='alice'`).Scan(&userID); err != nil {
		t.Fatalf("read user ID: %v", err)
	}
	userIDString := strconv.FormatInt(userID, 10)
	if _, err := db.Exec(`INSERT INTO user_settings(user_id,created_at,updated_at) VALUES(?,'now','now')`, userIDString); err != nil {
		t.Fatalf("insert settings using API string ID: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO plays(user_id,media_id,watched_at,source,created_at) VALUES(?,'movie:10','now','future-integration','now')`, userIDString); err != nil {
		t.Fatalf("insert play with open source value: %v", err)
	}
	for _, table := range []string{"user_settings", "plays"} {
		var storedType string
		if err := db.QueryRow(`SELECT typeof(user_id) FROM ` + table + ` LIMIT 1`).Scan(&storedType); err != nil {
			t.Fatalf("read stored %s.user_id type: %v", table, err)
		}
		if storedType != "integer" {
			t.Errorf("stored %s.user_id has type %s, want integer", table, storedType)
		}
	}
	var foreignKeyViolations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&foreignKeyViolations); err != nil {
		t.Fatalf("check foreign keys: %v", err)
	}
	if foreignKeyViolations != 0 {
		t.Fatalf("schema produced %d foreign-key violations", foreignKeyViolations)
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
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
