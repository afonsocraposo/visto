package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	modernsqlite "modernc.org/sqlite"
)

// Backup copies a live SQLite database to a new file using SQLite's online
// backup API. It does not overwrite an existing destination.
func Backup(ctx context.Context, sourcePath, destinationPath string) error {
	if sourcePath == "" || destinationPath == "" {
		return fmt.Errorf("source and destination paths are required")
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("inspect source database: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("source database must be a regular file")
	}
	destinationPath, err = filepath.Abs(destinationPath)
	if err != nil {
		return fmt.Errorf("resolve backup destination: %w", err)
	}
	if _, err := os.Lstat(destinationPath); err == nil {
		return fmt.Errorf("backup destination already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect backup destination: %w", err)
	}
	destinationDir := filepath.Dir(destinationPath)
	if err := os.MkdirAll(destinationDir, 0o750); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	temporaryDir, err := os.MkdirTemp(destinationDir, ".visto-backup-")
	if err != nil {
		return fmt.Errorf("create private backup directory: %w", err)
	}
	defer os.RemoveAll(temporaryDir)
	temporaryPath := filepath.Join(temporaryDir, "backup.db")
	temporaryURI := (&url.URL{Scheme: "file", Path: temporaryPath}).String()

	database, err := sql.Open("sqlite", filepath.Clean(sourcePath))
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	connection, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connect to source database: %w", err)
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("configure backup connection: %w", err)
	}
	if err := connection.Raw(func(driverConnection any) error {
		backuper, ok := driverConnection.(interface {
			NewBackup(string) (*modernsqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("SQLite driver does not support online backups")
		}
		backup, err := backuper.NewBackup(temporaryURI)
		if err != nil {
			return fmt.Errorf("start online backup: %w", err)
		}
		busyRetries := 0
		for more := true; more; {
			more, err = backup.Step(-1)
			if err != nil {
				var sqliteError interface{ Code() int }
				if errors.As(err, &sqliteError) && sqliteError.Code()&0xff == 5 && busyRetries < 50 {
					busyRetries++
					timer := time.NewTimer(100 * time.Millisecond)
					select {
					case <-ctx.Done():
						timer.Stop()
						_ = backup.Finish()
						return ctx.Err()
					case <-timer.C:
						continue
					}
				}
				_ = backup.Finish()
				return fmt.Errorf("copy database pages: %w", err)
			}
		}
		if err := backup.Finish(); err != nil {
			return fmt.Errorf("finish online backup: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return fmt.Errorf("secure backup file permissions: %w", err)
	}
	if err := os.Link(temporaryPath, destinationPath); err != nil {
		return fmt.Errorf("create backup without overwriting existing files: %w", err)
	}
	return nil
}
