package backup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

type Cipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}
type Service struct {
	Store                           *sqlite.Store
	Cipher                          Cipher
	DatabasePath, Directory         string
	DefaultInterval, LocalRetention time.Duration
	S3                              S3Client
	mu                              sync.Mutex
}

func (s *Service) Settings(ctx context.Context) (sqlite.BackupSettings, error) {
	b, err := s.Store.GetBackupSettings(ctx, s.DefaultInterval)
	b.EncryptionAvailable = s.Cipher != nil
	return b, err
}
func (s *Service) Save(ctx context.Context, b sqlite.BackupSettings, secret string) error {
	old, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	if b.Destination != "local" && b.Destination != "s3" && b.Destination != "both" {
		return errors.New("invalid backup destination")
	}
	if b.IntervalSeconds < 3600 || b.IntervalSeconds > 7*24*3600 {
		return errors.New("backup interval must be between 1 hour and 7 days")
	}
	if b.MaxKeep < 1 || b.MaxKeep > 1000 {
		return errors.New("S3 backup count must be between 1 and 1000")
	}
	b.Bucket = strings.TrimSpace(b.Bucket)
	b.Region = strings.TrimSpace(b.Region)
	b.Endpoint = strings.TrimSpace(b.Endpoint)
	b.AccessKeyID = strings.TrimSpace(b.AccessKeyID)
	b.Prefix = strings.Trim(b.Prefix, "/")
	if b.Prefix != "" {
		b.Prefix += "/"
	}
	if b.Endpoint != "" && !strings.HasPrefix(b.Endpoint, "https://") {
		return errors.New("S3 endpoint must use HTTPS")
	}
	if strings.Contains(b.Prefix, "..") || strings.ContainsAny(b.Prefix, "\\?") {
		return errors.New("invalid S3 prefix")
	}
	if secret != "" {
		if s.Cipher == nil {
			return errors.New("secret encryption is not configured")
		}
		b.SecretCiphertext, err = s.Cipher.Encrypt(secret)
		if err != nil {
			return err
		}
	} else {
		b.SecretCiphertext = old.SecretCiphertext
	}
	if b.Destination != "local" && (s.Cipher == nil || b.Bucket == "" || b.Region == "" || b.AccessKeyID == "" || b.SecretCiphertext == "") {
		return errors.New("S3 destination requires encryption, bucket, region, and access keys")
	}
	return s.Store.SaveBackupSettings(ctx, b)
}
func (s *Service) s3Config(b sqlite.BackupSettings) (S3Config, error) {
	if s.Cipher == nil {
		return S3Config{}, errors.New("secret encryption is not configured")
	}
	secret, err := s.Cipher.Decrypt(b.SecretCiphertext)
	if err != nil {
		return S3Config{}, err
	}
	return S3Config{Bucket: b.Bucket, Region: b.Region, Endpoint: b.Endpoint, AccessKeyID: b.AccessKeyID, SecretKey: secret, PathStyle: b.PathStyle, Prefix: b.Prefix, MaxKeep: b.MaxKeep}, nil
}
func (s *Service) Test(ctx context.Context) error {
	b, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	if b.SecretCiphertext == "" {
		return errors.New("save S3 credentials first")
	}
	cfg, err := s.s3Config(b)
	if err != nil {
		return err
	}
	return s.S3.Test(ctx, cfg)
}
func (s *Service) RunOnce(ctx context.Context, scheduled bool) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.run(ctx, scheduled)
	if recordErr := s.Store.RecordBackupResult(ctx, err, s.DefaultInterval); recordErr != nil {
		log.Printf("record backup result: %v", recordErr)
	}
	return err
}
func (s *Service) run(ctx context.Context, scheduled bool) error {
	b, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	name := backupNamePrefix + now.Format(backupNameLayout) + ".db"
	if !scheduled {
		name = "visto-manual-" + now.Format(backupNameLayout) + ".db"
	}
	var file string
	if b.Destination == "local" || b.Destination == "both" {
		if scheduled {
			file, err = Create(ctx, s.DatabasePath, s.Directory, s.LocalRetention, now)
		} else {
			file = filepath.Join(s.Directory, name)
			err = sqlite.Backup(ctx, s.DatabasePath, file)
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
		if err = sqlite.Backup(ctx, s.DatabasePath, file); err != nil {
			return err
		}
	}
	if b.Destination == "local" {
		return nil
	}
	cfg, err := s.s3Config(b)
	if err != nil {
		return err
	}
	if err = s.S3.Upload(ctx, cfg, cfg.Prefix+name, file); err != nil {
		return err
	}
	if scheduled {
		if err = s.S3.Prune(ctx, cfg); err != nil {
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
		settings, err := s.Settings(ctx)
		if err != nil {
			logger.Printf("read backup settings: %v", err)
		} else {
			interval := time.Duration(settings.IntervalSeconds) * time.Second
			if lastRun.IsZero() || time.Since(lastRun) >= interval {
				lastRun = time.Now()
				if err := s.RunOnce(ctx, true); err != nil {
					logger.Printf("automatic backup failed: %v", err)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
