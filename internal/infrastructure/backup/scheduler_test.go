package backup

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRun_GivenInvalidInterval_WhenStarted_ThenItReturnsWithoutStartingATicker(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	Run(context.Background(), "unused.db", t.TempDir(), 0, time.Hour, logger)
}

func TestPrune_GivenExpiredVistoBackupsAndOtherFiles_WhenRetentionRuns_ThenOnlyExpiredBackupsAreRemoved(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, time.January, 8, 12, 0, 0, 0, time.UTC)
	oldBackup := backupNamePrefix + now.Add(-8*24*time.Hour).Format(backupNameLayout) + ".db"
	freshBackup := backupNamePrefix + now.Add(-2*24*time.Hour).Format(backupNameLayout) + ".db"
	for _, name := range []string{oldBackup, freshBackup, "manual-copy.db"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := prune(directory, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("prune backups: %v", err)
	}

	if _, err := os.Stat(filepath.Join(directory, oldBackup)); !os.IsNotExist(err) {
		t.Fatalf("expired backup still exists or stat failed: %v", err)
	}
	for _, name := range []string{freshBackup, "manual-copy.db"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("backup %q should be preserved: %v", name, err)
		}
	}
}

func TestBackupDate_GivenNonManagedOrMalformedName_WhenParsed_ThenItIsIgnored(t *testing.T) {
	for _, name := range []string{"manual-copy.db", "visto-backup-invalid.db", "visto-backup-20260108T120000.000000000Z.sqlite"} {
		if _, ok := backupDate(name); ok {
			t.Errorf("backupDate(%q) unexpectedly accepted the name", name)
		}
	}
}
