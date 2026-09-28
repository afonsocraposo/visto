package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

// Service runs scheduled and manual SQLite backups using settings supplied
// entirely by the operator at startup (environment variables); it has no
// runtime-editable configuration.
type Service struct {
	DatabasePath, Directory         string
	Scope                           string
	Destination                     string
	DefaultInterval, LocalRetention time.Duration
	S3Config                        S3Config
	S3                              S3Client
	mu                              sync.Mutex
}

func (s *Service) scope() string {
	if s.Scope == "" {
		return "everything"
	}
	return s.Scope
}
func (s *Service) Test(ctx context.Context) error {
	return s.S3.Test(ctx, s.S3Config)
}
func (s *Service) RunOnce(ctx context.Context, scheduled bool) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.run(ctx, scheduled)
}
func (s *Service) run(ctx context.Context, scheduled bool) error {
	now := time.Now().UTC()
	name := backupNamePrefix + now.Format(backupNameLayout) + ".db"
	if !scheduled {
		name = "visto-manual-" + now.Format(backupNameLayout) + ".db"
	}
	var file string
	if s.Destination == "local" || s.Destination == "both" {
		var err error
		if scheduled {
			file, err = CreateWithScope(ctx, s.DatabasePath, s.Directory, s.LocalRetention, now, s.scope())
		} else {
			file = filepath.Join(s.Directory, name)
			err = sqlite.BackupWithScope(ctx, s.DatabasePath, file, s.scope())
		}
		if err != nil {
			return err
		}
	} else {
		dir, err := os.MkdirTemp("", "visto-s3-backup-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		file = filepath.Join(dir, name)
		if err := sqlite.BackupWithScope(ctx, s.DatabasePath, file, s.scope()); err != nil {
			return err
		}
	}
	if s.Destination == "local" {
		return nil
	}
	if err := s.S3.Upload(ctx, s.S3Config, s.S3Config.Prefix+name, file); err != nil {
		return err
	}
	if scheduled {
		if err := s.S3.Prune(ctx, s.S3Config); err != nil {
			return fmt.Errorf("S3 backup uploaded but cleanup failed: %w", err)
		}
	}
	return nil
}
func (s *Service) Run(ctx context.Context, logger *log.Logger) {
	if logger == nil {
		logger = log.Default()
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	lastRun := time.Time{}
	for {
		if lastRun.IsZero() || time.Since(lastRun) >= s.DefaultInterval {
			lastRun = time.Now()
			if err := s.RunOnce(ctx, true); err != nil {
				logger.Printf("automatic backup failed: %v", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
