package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var migrationFilename = regexp.MustCompile(`^(\d{4,})_([a-z0-9_]+)\.sql$`)

type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

type Migrator struct {
	Migrations []Migration
	Now        func() time.Time
}

func NewMigrator() (*Migrator, error) {
	migrations, err := LoadMigrations(migrationFiles)
	if err != nil {
		return nil, err
	}
	return &Migrator{Migrations: migrations, Now: time.Now}, nil
}

func LoadMigrations(files fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	versions := make(map[int]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFilename.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		if _, exists := versions[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d", version)
		}
		contents, err := fs.ReadFile(files, path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(contents))
		migrations = append(migrations, Migration{Version: version, Name: matches[2], SQL: string(contents), Checksum: checksum})
		versions[version] = struct{}{}
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	return migrations, nil
}

func (m *Migrator) Apply(ctx context.Context, db *sql.DB) error {
	if m.Now == nil {
		m.Now = time.Now
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedMigrations(ctx, db)
	if err != nil {
		return err
	}
	available := make(map[int]Migration, len(m.Migrations))
	for _, migration := range m.Migrations {
		available[migration.Version] = migration
	}
	for version, migration := range applied {
		file, exists := available[version]
		if !exists {
			return fmt.Errorf("applied migration %04d (%s) is missing from the binary", version, migration.Name)
		}
		if file.Checksum != migration.Checksum {
			return fmt.Errorf("checksum mismatch for applied migration %04d", version)
		}
	}

	for _, migration := range m.Migrations {
		if _, exists := applied[migration.Version]; exists {
			continue
		}
		if err := m.applyOne(ctx, db, migration); err != nil {
			return err
		}
	}
	return nil
}

type appliedMigration struct {
	Name     string
	Checksum string
}

func appliedMigrations(ctx context.Context, db *sql.DB) (map[int]appliedMigration, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]appliedMigration)
	for rows.Next() {
		var version int
		var migration appliedMigration
		if err := rows.Scan(&version, &migration.Name, &migration.Checksum); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = migration
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

func (m *Migrator) applyOne(ctx context.Context, db *sql.DB, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %04d: %w", migration.Version, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %04d_%s: %w", migration.Version, migration.Name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES (?, ?, ?, ?)`,
		migration.Version, migration.Name, migration.Checksum, m.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("record migration %04d: %w", migration.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %04d: %w", migration.Version, err)
	}
	return nil
}
