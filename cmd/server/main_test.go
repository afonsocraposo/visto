package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestRunBackup_GivenServerDatabase_WhenOperatorRequestsBackup_ThenTheBackupCanBeRead(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	store, err := sqlite.Open(context.Background(), sourcePath)
	if err != nil {
		t.Fatalf("open source database: %v", err)
	}
	if _, err := store.DB.Exec(`CREATE TABLE backup_smoke (value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO backup_smoke VALUES ('saved')`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	destinationPath := filepath.Join(t.TempDir(), "backup.db")
	if err := runBackup(sourcePath, destinationPath); err != nil {
		t.Fatalf("run backup command: %v", err)
	}
	backup, err := sql.Open("sqlite", destinationPath)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backup.Close()
	var value string
	if err := backup.QueryRow(`SELECT value FROM backup_smoke`).Scan(&value); err != nil {
		t.Fatalf("read backup data: %v", err)
	}
	if value != "saved" {
		t.Fatalf("backup value=%q, want saved", value)
	}
}

func TestParseDuration_GivenEmptyValidOrInvalidSetting_WhenParsed_ThenItUsesOnlyPositiveDurations(t *testing.T) {
	fallback := 6 * time.Hour
	if actual, err := parseDuration("VISTO_TEST_INTERVAL", "", fallback); err != nil || actual != fallback {
		t.Fatalf("empty setting = %s, %v; want fallback %s", actual, err, fallback)
	}
	if actual, err := parseDuration("VISTO_TEST_INTERVAL", "12h", fallback); err != nil || actual != 12*time.Hour {
		t.Fatalf("valid setting = %s, %v; want 12h", actual, err)
	}
	for _, value := range []string{"0s", "-1h", "invalid"} {
		if _, err := parseDuration("VISTO_TEST_INTERVAL", value, fallback); err == nil {
			t.Errorf("setting %q was accepted, want a positive duration error", value)
		}
	}
}
