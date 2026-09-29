package sqlite_test

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	_ "modernc.org/sqlite"
)

func TestMigrator_GivenFreshDatabase_WhenAppliedTwice_ThenCurrentSchemaExistsAndSecondRunIsNoOp(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	if len(migrator.Migrations) != 14 || migrator.Migrations[0].Version != 1 || migrator.Migrations[13].Version != 14 {
		t.Fatalf("loaded migrations = %#v, want versions 1 through 14", migrator.Migrations)
	}
	migrator.Now = func() time.Time { return time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC) }

	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 14 {
		t.Fatalf("migration count = %d, want 14", count)
	}
	var version int
	var name string
	if err := db.QueryRow(`SELECT version, name FROM schema_migrations WHERE version=1`).Scan(&version, &name); err != nil {
		t.Fatalf("read baseline migration: %v", err)
	}
	if version != 1 || name != "initial_schema" {
		t.Fatalf("applied migration = (%d, %q), want (1, initial_schema)", version, name)
	}

	for _, table := range []string{
		"users", "user_settings", "sessions", "media", "seasons", "episodes", "user_media", "plays",
		"activity_events", "episode_ratings", "personal_api_tokens", "notification_deliveries", "movie_release_deliveries", "oauth_clients",
		"oauth_authorization_codes", "oauth_access_tokens", "oauth_refresh_tokens", "plex_webhooks", "plex_webhook_events", "plex_observed_accounts",
	} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %q: %v", table, err)
		}
	}

	for _, table := range []string{"users", "sessions", "plays", "activity_events", "personal_api_tokens"} {
		var columnType string
		var primaryKey int
		if err := db.QueryRow(`SELECT type, pk FROM pragma_table_info(?) WHERE name = 'id'`, table).Scan(&columnType, &primaryKey); err != nil {
			t.Fatalf("read %s.id schema: %v", table, err)
		}
		if !strings.EqualFold(columnType, "INTEGER") || primaryKey != 1 {
			t.Errorf("%s.id = (%s, pk=%d), want INTEGER primary key", table, columnType, primaryKey)
		}
	}
	for _, table := range []string{
		"user_settings", "sessions", "user_media", "plays", "activity_events", "episode_ratings",
		"personal_api_tokens", "notification_deliveries", "oauth_authorization_codes", "oauth_access_tokens",
		"oauth_refresh_tokens", "plex_webhooks", "plex_webhook_events",
	} {
		var columnType string
		if err := db.QueryRow(`SELECT type FROM pragma_table_info(?) WHERE name = 'user_id'`, table).Scan(&columnType); err != nil {
			t.Fatalf("read %s.user_id schema: %v", table, err)
		}
		if !strings.EqualFold(columnType, "INTEGER") {
			t.Errorf("%s.user_id has type %s, want INTEGER", table, columnType)
		}
	}

	for _, check := range []struct{ table, column string }{
		{"media", "catalog_updated_at"},
		{"oauth_access_tokens", "last_used_at"},
		{"activity_events", "created_at"},
	} {
		var column string
		if err := db.QueryRow(`SELECT name FROM pragma_table_info(?) WHERE name = ?`, check.table, check.column).Scan(&column); err != nil {
			t.Errorf("expected %s.%s: %v", check.table, check.column, err)
		}
	}
	var index string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_activity_events_created_at'`).Scan(&index); err != nil {
		t.Fatalf("expected activity-event retention index: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO users(username,display_name,password_hash,role,created_at,updated_at) VALUES('alice','Alice','hash','user','now','now')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie','now','now')`); err != nil {
		t.Fatalf("insert media: %v", err)
	}
	var userID int64
	if err := db.QueryRow(`SELECT id FROM users WHERE username='alice'`).Scan(&userID); err != nil {
		t.Fatalf("read user ID: %v", err)
	}
	userIDString := strconv.FormatInt(userID, 10)
	if _, err := db.Exec(`INSERT INTO user_settings(user_id,created_at,updated_at) VALUES(?,'now','now')`, userIDString); err != nil {
		t.Fatalf("insert settings using API string ID: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO plays(user_id,media_id,watched_at,source,created_at) VALUES(?,'movie:10','now','future-integration','now')`, userIDString); err != nil {
		t.Fatalf("insert play with open source value: %v", err)
	}
	for _, table := range []string{"user_settings", "plays"} {
		var storedType string
		if err := db.QueryRow(`SELECT typeof(user_id) FROM ` + table + ` LIMIT 1`).Scan(&storedType); err != nil {
			t.Fatalf("read stored %s.user_id type: %v", table, err)
		}
		if storedType != "integer" {
			t.Errorf("stored %s.user_id has type %s, want integer", table, storedType)
		}
	}
	var foreignKeyViolations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&foreignKeyViolations); err != nil {
		t.Fatalf("check foreign keys: %v", err)
	}
	if foreignKeyViolations != 0 {
		t.Fatalf("schema produced %d foreign-key violations", foreignKeyViolations)
	}
}

func TestMigrator_GivenExistingPersonalWebhook_WhenUpgraded_ThenItsTokenIsPreserved(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatal(err)
	}
	legacy := &sqlite.Migrator{Migrations: migrator.Migrations[:10], Now: time.Now}
	if err := legacy.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	userID := insertTestUser(t, db, "plex-owner", "Plex Owner", "private")
	if _, err := db.Exec(`INSERT INTO plex_webhooks(user_id,token_hash,account_id,created_at) VALUES(?,?,?,?)`, userID, "old-hash", "123", testTimestamp); err != nil {
		t.Fatal(err)
	}
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var hash, accountID string
	if err := db.QueryRow(`SELECT token_hash,account_id FROM plex_webhooks WHERE user_id=?`, userID).Scan(&hash, &accountID); err != nil {
		t.Fatal(err)
	}
	if hash != "old-hash" || accountID != "123" {
		t.Fatalf("preserved webhook hash=%q account=%q", hash, accountID)
	}
}

func TestMigrator_GivenChangedAppliedMigration_WhenApplied_ThenItFails(t *testing.T) {
	db := openTestDatabase(t)
	first := &sqlite.Migrator{Migrations: []sqlite.Migration{{Version: 1, Name: "create_items", SQL: "CREATE TABLE items (id INTEGER);", Checksum: "first"}}, Now: time.Now}
	if err := first.Apply(context.Background(), db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	changed := &sqlite.Migrator{Migrations: []sqlite.Migration{{Version: 1, Name: "create_items", SQL: "CREATE TABLE items (id INTEGER, name TEXT);", Checksum: "changed"}}, Now: time.Now}

	err := changed.Apply(context.Background(), db)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want checksum mismatch", err)
	}
}

func TestLoadMigrations_GivenInvalidFilename_WhenLoaded_ThenItFails(t *testing.T) {
	_, err := sqlite.LoadMigrations(fstest.MapFS{"migrations/not-a-migration.txt": {Data: []byte("SELECT 1;")}})
	if err == nil || !strings.Contains(err.Error(), "invalid migration filename") {
		t.Fatalf("error = %v, want invalid filename error", err)
	}
}

func TestCompletedStatusMigration_PreservesHistoryAndNormalizesLists(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrator.Migrations[:6] {
		if _, err := db.Exec(migration.SQL); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at)
		VALUES(1,'u','User','hash','user','now','now');
		INSERT INTO media(id,media_type,tmdb_id,title,status,metadata_updated_at,created_at) VALUES
		('movie:1','movie',1,'Watched Movie','','now','now'),
		('movie:2','movie',2,'Saved Movie','','now','now'),
		('tv:3','tv',3,'Started Show','Returning Series','now','now'),
		('tv:4','tv',4,'Finished Show','Ended','now','now'),
		('tv:5','tv',5,'Unlisted Finished Show','Ended','now','now');
		INSERT INTO seasons(id,show_id,season_number,name,episode_count) VALUES
		('tv:3:season:1','tv:3',1,'Season 1',2),('tv:4:season:1','tv:4',1,'Season 1',1),
		('tv:5:season:1','tv:5',1,'Season 1',1);
		INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES
		('e31','tv:3','tv:3:season:1',1,1,'One'),('e41','tv:4','tv:4:season:1',1,1,'One'),
		('e51','tv:5','tv:5:season:1',1,1,'One');
		INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at) VALUES
		('u:m1',1,'movie:1','watching',5,'old','old'),('u:m2',1,'movie:2','watching',NULL,'old','old'),
		('u:t3',1,'tv:3','watchlist',NULL,'old','old'),('u:t4',1,'tv:4','paused',NULL,'old','old');
		INSERT INTO plays(user_id,media_id,episode_id,watched_at,source,created_at) VALUES
		(1,'movie:1',NULL,'old','web','old'),(1,NULL,'e31','old','web','old'),(1,NULL,'e41','old','web','old'),
		(1,NULL,'e51','old','web','2026-09-27T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migrator.Migrations[6].SQL); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"movie:1": "completed", "movie:2": "watchlist", "tv:3": "watching", "tv:4": "completed", "tv:5": "completed"}
	rows, err := db.Query(`SELECT media_id,status FROM user_media ORDER BY media_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			t.Fatal(err)
		}
		if status != want[id] {
			t.Fatalf("%s status=%s want %s", id, status, want[id])
		}
		delete(want, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(want) != 0 {
		t.Fatalf("missing entries: %v", want)
	}
	var rating, plays int
	if err := db.QueryRow(`SELECT rating FROM user_media WHERE media_id='movie:1'`).Scan(&rating); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM plays`).Scan(&plays); err != nil {
		t.Fatal(err)
	}
	if rating != 5 || plays != 4 {
		t.Fatalf("history changed: rating=%d plays=%d", rating, plays)
	}
}

func TestImportedShowCompletionMigrationRepairsWatchingOnly(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatal(err)
	}
	old := *migrator
	old.Migrations = migrator.Migrations[:9]
	if err := old.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at)
		VALUES(1,'u','User','hash','user','now','now');
		INSERT INTO media(id,media_type,tmdb_id,title,status,metadata_updated_at,created_at)
		VALUES('tv:1','tv',1,'Finished','Ended','now','now'),('tv:2','tv',2,'Unfinished','Ended','now','now');
		INSERT INTO seasons(id,show_id,season_number,name,episode_count)
		VALUES('s1','tv:1',1,'Season 1',1),('s2','tv:2',1,'Season 1',2);
		INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name)
		VALUES('e1','tv:1','s1',1,1,'One'),('e2','tv:2','s2',1,1,'One'),('e3','tv:2','s2',1,2,'Two');
		INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at)
		VALUES('a',1,'tv:1','watching','old','old'),('b',1,'tv:2','watching','old','old');
		INSERT INTO plays(user_id,episode_id,watched_at,source,created_at)
		VALUES(1,'e1','old','import','old'),(1,'e2','old','import','old');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for mediaID, want := range map[string]string{"tv:1": "completed", "tv:2": "watching"} {
		var status string
		if err := db.QueryRow(`SELECT status FROM user_media WHERE media_id=?`, mediaID).Scan(&status); err != nil || status != want {
			t.Fatalf("%s status=%q, want %q, err=%v", mediaID, status, want, err)
		}
	}
}

func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestWebPushMigrationPreservesPushoverDelivery(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatal(err)
	}
	old := *migrator
	old.Migrations = migrator.Migrations[:3]
	if err := old.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(1,'alex','Alex','hash','user','2026-09-01','2026-09-01');
 INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:1','tv',1,'Show','2026-09-01','2026-09-01');
 INSERT INTO seasons(id,show_id,season_number) VALUES('season','tv:1',1);
 INSERT INTO episodes(id,show_id,season_id,season_number,episode_number) VALUES('episode','tv:1','season',1,1);
 INSERT INTO notification_deliveries(user_id,episode_id,state,attempted_at,attempt_count,sent_at) VALUES(1,'episode','sent','2026-09-01',1,'2026-09-01');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM notification_deliveries WHERE user_id=1 AND episode_id='episode'`).Scan(&state); err != nil || state != "sent" {
		t.Fatalf("state=%s err=%v", state, err)
	}
}
