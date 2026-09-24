package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestBackup_GivenLiveDatabase_WhenBackupIsCreated_ThenItContainsTheCommittedDataAndIsPrivate(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	store, err := sqlite.Open(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`CREATE TABLE backup_fixture (value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO backup_fixture(value) VALUES('saved')`); err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	destinationPath := filepath.Join(t.TempDir(), "backups", "visto.db")
	if err := sqlite.Backup(ctx, sourcePath, destinationPath); err != nil {
		t.Fatalf("backup database: %v", err)
	}
	backup, err := sql.Open("sqlite", destinationPath)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var value string
	if err := backup.QueryRow(`SELECT value FROM backup_fixture`).Scan(&value); err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if value != "saved" {
		t.Fatalf("backup value=%q, want saved", value)
	}
	info, err := os.Stat(destinationPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("backup permissions=%#o, want 0600", info.Mode().Perm())
	}
}

func TestBackup_GivenExistingDestination_WhenBackupIsRequested_ThenItDoesNotOverwriteTheFile(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	store, err := sqlite.Open(context.Background(), sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	destinationPath := filepath.Join(t.TempDir(), "existing.db")
	if err := os.WriteFile(destinationPath, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	err = sqlite.Backup(context.Background(), sourcePath, destinationPath)
	if err == nil {
		t.Fatal("expected existing destination to be rejected")
	}
	contents, err := os.ReadFile(destinationPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "keep me" {
		t.Fatalf("destination contents=%q, want unchanged file", contents)
	}
}

func TestBackup_GivenMissingSource_WhenBackupIsRequested_ThenItDoesNotCreateAReplacementDatabase(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "missing.db")
	destinationPath := filepath.Join(t.TempDir(), "backup.db")
	if err := sqlite.Backup(context.Background(), sourcePath, destinationPath); err == nil {
		t.Fatal("expected missing source database to be rejected")
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source path should remain absent, stat error=%v", err)
	}
}
