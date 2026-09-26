package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type BackupSettings struct {
	Destination         string `json:"destination"`
	IntervalSeconds     int64  `json:"interval_seconds"`
	Bucket              string `json:"bucket"`
	Region              string `json:"region"`
	Endpoint            string `json:"endpoint"`
	AccessKeyID         string `json:"access_key_id"`
	SecretCiphertext    string `json:"-"`
	PathStyle           bool   `json:"path_style"`
	Prefix              string `json:"prefix"`
	MaxKeep             int    `json:"max_keep"`
	LastSuccessAt       string `json:"last_success_at"`
	LastError           string `json:"last_error"`
	HasSecret           bool   `json:"has_secret"`
	EncryptionAvailable bool   `json:"encryption_available"`
}

func (s *Store) GetBackupSettings(ctx context.Context, fallback time.Duration) (BackupSettings, error) {
	b := BackupSettings{Destination: "local", IntervalSeconds: int64(fallback.Seconds()), Prefix: "visto/", MaxKeep: 30}
	var style int
	err := s.DB.QueryRowContext(ctx, `SELECT destination,interval_seconds,s3_bucket,s3_region,s3_endpoint,s3_access_key_id,s3_secret_ciphertext,s3_path_style,s3_prefix,s3_max_keep,last_success_at,last_error FROM backup_settings WHERE id=1`).Scan(&b.Destination, &b.IntervalSeconds, &b.Bucket, &b.Region, &b.Endpoint, &b.AccessKeyID, &b.SecretCiphertext, &style, &b.Prefix, &b.MaxKeep, &b.LastSuccessAt, &b.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return b, nil
	}
	if err != nil {
		return b, err
	}
	b.PathStyle = style != 0
	b.HasSecret = b.SecretCiphertext != ""
	return b, nil
}
func (s *Store) SaveBackupSettings(ctx context.Context, b BackupSettings) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO backup_settings(id,destination,interval_seconds,s3_bucket,s3_region,s3_endpoint,s3_access_key_id,s3_secret_ciphertext,s3_path_style,s3_prefix,s3_max_keep) VALUES(1,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET destination=excluded.destination,interval_seconds=excluded.interval_seconds,s3_bucket=excluded.s3_bucket,s3_region=excluded.s3_region,s3_endpoint=excluded.s3_endpoint,s3_access_key_id=excluded.s3_access_key_id,s3_secret_ciphertext=excluded.s3_secret_ciphertext,s3_path_style=excluded.s3_path_style,s3_prefix=excluded.s3_prefix,s3_max_keep=excluded.s3_max_keep`, b.Destination, b.IntervalSeconds, b.Bucket, b.Region, b.Endpoint, b.AccessKeyID, b.SecretCiphertext, b.PathStyle, b.Prefix, b.MaxKeep)
	return err
}
func (s *Store) RecordBackupResult(ctx context.Context, runErr error, fallback time.Duration) error {
	if runErr != nil {
		_, err := s.DB.ExecContext(ctx, `INSERT INTO backup_settings(id,destination,interval_seconds,last_error) VALUES(1,'local',?,?) ON CONFLICT(id) DO UPDATE SET last_error=excluded.last_error`, int64(fallback.Seconds()), runErr.Error())
		return err
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO backup_settings(id,destination,interval_seconds,last_success_at) VALUES(1,'local',?,?) ON CONFLICT(id) DO UPDATE SET last_success_at=excluded.last_success_at,last_error=''`, int64(fallback.Seconds()), time.Now().UTC().Format(time.RFC3339))
	return err
}
