package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// trimMetadata changes only the private backup copy. Referenced catalogue rows
// remain as identity records so all user-owned foreign keys survive restore.
func trimMetadata(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`DELETE FROM episodes WHERE id NOT IN (
			SELECT episode_id FROM plays WHERE episode_id IS NOT NULL UNION
			SELECT episode_id FROM activity_events WHERE episode_id IS NOT NULL UNION
			SELECT episode_id FROM episode_ratings UNION
			SELECT episode_id FROM notification_deliveries UNION
			SELECT episode_id FROM web_push_deliveries)`,
		`DELETE FROM seasons WHERE id NOT IN (SELECT season_id FROM episodes UNION SELECT season_id FROM season_ready_alerts)`,
		`DELETE FROM media WHERE id NOT IN (
			SELECT media_id FROM user_media UNION
			SELECT media_id FROM plays WHERE media_id IS NOT NULL UNION
			SELECT media_id FROM activity_events WHERE media_id IS NOT NULL UNION
			SELECT show_id FROM episodes UNION
			SELECT show_id FROM seasons)`,
		`UPDATE media SET title=media_type || ':' || tmdb_id,
			original_title=NULL,overview=NULL,release_date=NULL,status=NULL,
			poster_path=NULL,backdrop_path=NULL,original_language=NULL,
			raw_metadata=NULL,metadata_updated_at='',catalog_updated_at=NULL`,
		`UPDATE seasons SET tmdb_id=NULL,name=NULL,overview=NULL,poster_path=NULL,
			air_date=NULL,episode_count=NULL`,
		`UPDATE episodes SET name=NULL,overview=NULL,air_date=NULL,runtime=NULL,
			still_path=NULL,raw_metadata=NULL,metadata_updated_at=NULL`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("trim catalogue: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("backup has invalid foreign keys")
	}
	return rows.Err()
}
