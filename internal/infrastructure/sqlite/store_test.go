package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestOpen_GivenNewDatabasePath_WhenOpened_ThenItMigratesAndEnablesForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "visto.db")
	store, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var foreignKeys int
	if err := store.DB.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign key setting: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	var migrations int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrations); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrations != 1 {
		t.Fatalf("migrations = %d, want 1", migrations)
	}
}
