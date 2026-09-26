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

func TestBackupWithScope_PreservesUserLinksWithoutTMDBMetadata(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "source.db")
	store, err := sqlite.Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.DB.Exec(`
		INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(1,'alice','Alice','hash','admin','now','now');
		INSERT INTO media(id,media_type,tmdb_id,title,overview,metadata_updated_at,created_at) VALUES
		('tv:1','tv',1,'Saved title','TMDB description','now','now'),
		('movie:2','movie',2,'Unrelated title','Other description','now','now');
		INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:1:season:1','tv:1',1,'Season');
		INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:1:episode:11','tv:1','tv:1:season:1',1,1,'Episode');
		INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES('saved',1,'tv:1','watching','now','now');
		INSERT INTO plays(user_id,episode_id,watched_at,created_at) VALUES(1,'tv:1:episode:11','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "backup.db")
	if err := sqlite.BackupWithScope(ctx, source, destination, "user_data"); err != nil {
		t.Fatal(err)
	}
	restored, err := sqlite.Open(ctx, destination)
	if err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	defer restored.Close()
	var title string
	if err := restored.DB.QueryRow(`SELECT title FROM media WHERE id='tv:1'`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "tv:1" {
		t.Fatalf("unexpected retained metadata: %q", title)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM media WHERE id='movie:2'`,
		`SELECT COUNT(*) FROM user_media WHERE media_id='tv:1'`,
		`SELECT COUNT(*) FROM plays WHERE episode_id='tv:1:episode:11'`,
		`SELECT COUNT(*) FROM seasons WHERE id='tv:1:season:1'`,
	} {
		var count int
		if err := restored.DB.QueryRow(query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		want := 1
		if query == `SELECT COUNT(*) FROM media WHERE id='movie:2'` {
			want = 0
		}
		if count != want {
			t.Fatalf("%s: got %d, want %d", query, count, want)
		}
	}
	var violations int
	if err := restored.DB.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
		t.Fatal(err)
	}
	if violations != 0 {
		t.Fatalf("foreign key violations: %d", violations)
	}
}
