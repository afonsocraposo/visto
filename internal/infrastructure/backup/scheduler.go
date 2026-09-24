package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

const backupNamePrefix = "visto-backup-"
const backupNameLayout = "20060102T150405.000000000Z"

// Create writes a consistent online backup and then removes expired backups
// created by Visto in the configured backup directory.
func Create(ctx context.Context, databasePath, directory string, retention time.Duration, now time.Time) (string, error) {
	if databasePath == "" || directory == "" {
		return "", fmt.Errorf("database and backup paths are required")
	}
	if retention <= 0 {
		return "", fmt.Errorf("backup retention must be positive")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	now = now.UTC()
	filename := backupNamePrefix + now.Format(backupNameLayout) + ".db"
	path := filepath.Join(directory, filename)
	if err := sqlite.Backup(ctx, databasePath, path); err != nil {
		return "", fmt.Errorf("create SQLite backup: %w", err)
	}
	if err := prune(directory, now.Add(-retention)); err != nil {
		return path, fmt.Errorf("prune expired backups: %w", err)
	}
	return path, nil
}

// Run keeps one process-local backup schedule. Each run creates a new backup;
// expired backups are pruned only after a successful backup.
func Run(ctx context.Context, databasePath, directory string, interval, retention time.Duration, logger *log.Logger) {
	if logger == nil {
		logger = log.Default()
	}
	if interval <= 0 {
		logger.Printf("automatic database backup interval must be positive")
		return
	}
	run := func() {
		path, err := Create(ctx, databasePath, directory, retention, time.Now())
		if err != nil {
			logger.Printf("automatic database backup failed: %v", err)
			return
		}
		logger.Printf("automatic database backup created at %s", path)
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func prune(directory string, cutoff time.Time) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read backup directory: %w", err)
	}
	type oldBackup struct {
		path string
		date time.Time
	}
	var expired []oldBackup
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			continue
		}
		date, ok := backupDate(entry.Name())
		if !ok || !date.Before(cutoff) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect backup %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		expired = append(expired, oldBackup{path: filepath.Join(directory, entry.Name()), date: date})
	}
	sort.Slice(expired, func(i, j int) bool { return expired[i].date.Before(expired[j].date) })
	for _, backup := range expired {
		if err := os.Remove(backup.path); err != nil {
			return fmt.Errorf("remove expired backup %q: %w", filepath.Base(backup.path), err)
		}
	}
	return nil
}

func backupDate(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, backupNamePrefix) || !strings.HasSuffix(name, ".db") {
		return time.Time{}, false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(name, backupNamePrefix), ".db")
	date, err := time.Parse(backupNameLayout, value)
	return date, err == nil
}
