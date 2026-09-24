package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(ctx context.Context, databasePath string) (*Store, error) {
	if databasePath == "" {
		return nil, fmt.Errorf("database path is required")
	}
	databasePath = filepath.Clean(databasePath)
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	store := &Store{DB: db}
	if err := store.configure(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	migrator, err := NewMigrator()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrator.Apply(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate SQLite database: %w", err)
	}
	return store, nil
}

func (store *Store) configure(ctx context.Context) error {
	store.DB.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := store.DB.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure SQLite (%s): %w", pragma, err)
		}
	}
	return nil
}

func (store *Store) Close() error {
	return store.DB.Close()
}
