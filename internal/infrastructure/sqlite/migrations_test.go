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
	if count != 3 {
		t.Fatalf("migration count = %d, want 3", count)
	}
	for _, table := range []string{"users", "media", "episodes", "plays", "activity_events", "episode_ratings"} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %q: %v", table, err)
		}
	}
	var column string
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('media') WHERE name='catalog_updated_at'`).Scan(&column); err != nil {
		t.Fatalf("expected catalog refresh timestamp migration: %v", err)
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
