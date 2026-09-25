package sqlite_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	_ "modernc.org/sqlite"
)

func TestMigrator_GivenFreshDatabase_WhenAppliedTwice_ThenSchemaIsCreatedAndSecondRunIsANoOp(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatalf("new migrator: %v", err)
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
	if count != len(migrator.Migrations) {
		t.Fatalf("migration count = %d, want %d", count, len(migrator.Migrations))
	}
	for _, table := range []string{"users", "media", "episodes", "plays", "activity_events", "episode_ratings", "personal_api_tokens", "notification_deliveries", "oauth_clients", "oauth_authorization_codes", "oauth_access_tokens", "oauth_refresh_tokens", "plex_webhooks", "plex_webhook_events"} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %q: %v", table, err)
		}
	}
	var column string
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('media') WHERE name='catalog_updated_at'`).Scan(&column); err != nil {
		t.Fatalf("expected catalog refresh timestamp migration: %v", err)
	}
	if err := db.QueryRow(`SELECT name FROM pragma_table_info('oauth_access_tokens') WHERE name='last_used_at'`).Scan(&column); err != nil {
		t.Fatalf("expected OAuth connection metadata migration: %v", err)
	}
	var index string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_activity_events_created_at'`).Scan(&index); err != nil {
		t.Fatalf("expected activity-event retention index migration: %v", err)
	}
}

func TestMigrator_GivenExistingPlaysAndFeedActivity_WhenPlaySourceConstraintIsRemoved_ThenMigrationPreservesBoth(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatal(err)
	}
	migrations := migrator.Migrations
	migrator.Migrations = migrations[:11]
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("apply original schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES('alice','alice','Alice','hash','user',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO plays(id,user_id,media_id,watched_at,source,created_at) VALUES('play-1','alice','movie:10',?,'web',?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO activity_events(id,user_id,kind,play_id,media_id,occurred_at,created_at) VALUES('activity-1','alice','watch','play-1','movie:10',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	migrator.Migrations = migrations
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("apply open play source migration: %v", err)
	}
	var source, playID string
	if err := db.QueryRow(`SELECT p.source,CAST(a.play_id AS TEXT) FROM plays p JOIN activity_events a ON a.play_id=p.id WHERE p.media_id='movie:10'`).Scan(&source, &playID); err != nil {
		t.Fatalf("existing play/feed row was not preserved: %v", err)
	}
	if source != "web" || playID == "" || playID == "play-1" {
		t.Fatalf("preserved source=%q play ID=%q", source, playID)
	}
	if _, err := db.Exec(`UPDATE plays SET source='future-integration' WHERE id=?`, playID); err != nil {
		t.Fatalf("new source was rejected: %v", err)
	}
}

func TestMigrator_GivenExistingAccountsAndWatchHistory_WhenSerialKeysAreApplied_ThenDataAndForeignKeysArePreserved(t *testing.T) {
	db := openTestDatabase(t)
	migrator, err := sqlite.NewMigrator()
	if err != nil {
		t.Fatal(err)
	}
	migrations := migrator.Migrations
	migrator.Migrations = migrations[:13]
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("apply schema before serial keys: %v", err)
	}
	fixtures := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at,email) VALUES('legacy-user','legacy@example.test','Legacy User','hash','user',?,?,'legacy@example.test')`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO user_settings(user_id,created_at,updated_at) VALUES('legacy-user',?,?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO sessions(id,user_id,token_hash,expires_at,created_at) VALUES('session-old','legacy-user','session-hash','2030-01-01','2026-01-01')`, nil},
		{`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:77','movie',77,'Legacy Movie',?,?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:88','tv',88,'Legacy Show',?,?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:88:season:1','tv:88',1,'Season 1')`, nil},
		{`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:88:episode:1:1','tv:88','tv:88:season:1',1,1,'Pilot')`, nil},
		{`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES('legacy-user:movie:77','legacy-user','movie:77','watching',?,?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO plays(id,user_id,media_id,watched_at,source,created_at) VALUES('play-old','legacy-user','movie:77',?,'web',?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO activity_events(id,user_id,kind,play_id,media_id,occurred_at,created_at) VALUES('activity-old','legacy-user','watch','play-old','movie:77',?,?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO plays(id,user_id,media_id,watched_at,source,created_at) VALUES('play-old-2','legacy-user','movie:77',?,'web',?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO activity_events(id,user_id,kind,media_id,detail_json,occurred_at,created_at) VALUES('bulk-old','legacy-user','bulk_watch','movie:77','{"count":2,"play_ids":["play-old","play-old-2"]}',?,?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO episode_ratings(user_id,episode_id,rating,created_at,updated_at) VALUES('legacy-user','tv:88:episode:1:1',5,?,?)`, []any{testTimestamp, testTimestamp}},
		{`INSERT INTO notification_deliveries(user_id,episode_id,state,attempted_at) VALUES('legacy-user','tv:88:episode:1:1','sent',?)`, []any{testTimestamp}},
		{`INSERT INTO personal_api_tokens(id,user_id,name,token_hash,created_at) VALUES('token-old','legacy-user','Legacy token','token-hash',?)`, []any{testTimestamp}},
		{`INSERT INTO oauth_clients(client_id,client_name,redirect_uris_json,created_at) VALUES('client-old','Legacy client','[]',?)`, []any{testTimestamp}},
		{`INSERT INTO oauth_authorization_codes(code_hash,client_id,user_id,redirect_uri,code_challenge,scope,resource,expires_at,created_at) VALUES('code-hash','client-old','legacy-user','https://example.test/callback','challenge','read','https://example.test/mcp','2030-01-01',?)`, []any{testTimestamp}},
		{`INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,resource,expires_at) VALUES('access-hash','client-old','legacy-user','read','https://example.test/mcp','2030-01-01')`, nil},
		{`INSERT INTO oauth_refresh_tokens(token_hash,client_id,user_id,scope,resource,expires_at) VALUES('refresh-hash','client-old','legacy-user','read','https://example.test/mcp','2030-01-01')`, nil},
		{`INSERT INTO plex_webhooks(user_id,token_hash,created_at) VALUES('legacy-user','plex-hash',?)`, []any{testTimestamp}},
		{`INSERT INTO plex_webhook_events(user_id,fingerprint,status,occurred_at,created_at) VALUES('legacy-user','fingerprint','synced',?,?)`, []any{testTimestamp, testTimestamp}},
	}
	for index, fixture := range fixtures {
		if _, err := db.Exec(fixture.query, fixture.args...); err != nil {
			t.Fatalf("insert legacy fixture %d: %v", index, err)
		}
	}
	migrator.Migrations = migrations
	if err := migrator.Apply(context.Background(), db); err != nil {
		t.Fatalf("apply serial-key migration: %v", err)
	}

	var userID, sessionID, sessionUserID, mediaKey, playID, eventPlayID, tokenID, tokenUserID string
	var userIDType, sessionIDType, playIDType, eventIDType, tokenIDType string
	if err := db.QueryRow(`SELECT CAST(id AS TEXT),typeof(id) FROM users WHERE email='legacy@example.test'`).Scan(&userID, &userIDType); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT CAST(id AS TEXT),typeof(id),CAST(user_id AS TEXT) FROM sessions WHERE token_hash='session-hash'`).Scan(&sessionID, &sessionIDType, &sessionUserID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id,typeof(id),media_id FROM plays WHERE media_id='movie:77'`).Scan(&playID, &playIDType, &mediaKey); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT typeof(id),CAST(play_id AS TEXT) FROM activity_events WHERE kind='watch'`).Scan(&eventIDType, &eventPlayID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT CAST(id AS TEXT),typeof(id),CAST(user_id AS TEXT) FROM personal_api_tokens WHERE token_hash='token-hash'`).Scan(&tokenID, &tokenIDType, &tokenUserID); err != nil {
		t.Fatal(err)
	}
	if userID == "legacy-user" || userID == "" || sessionID == "session-old" || sessionID == "" || tokenID == "token-old" || tokenID == "" || userIDType != "integer" || sessionIDType != "integer" || playIDType != "integer" || eventIDType != "integer" || tokenIDType != "integer" {
		t.Fatalf("serial keys not applied: IDs users=%q sessions=%q plays=%q events=%q tokens=%q; types users=%q sessions=%q plays=%q events=%q tokens=%q", userID, sessionID, playID, eventPlayID, tokenID, userIDType, sessionIDType, playIDType, eventIDType, tokenIDType)
	}
	if sessionUserID != userID || tokenUserID != userID || mediaKey != "movie:77" || eventPlayID != playID {
		t.Fatalf("references not preserved: user=%q session user=%q token user=%q media=%q play=%q event play=%q", userID, sessionUserID, tokenUserID, mediaKey, playID, eventPlayID)
	}
	var bulkPlayIDs string
	if err := db.QueryRow(`SELECT json_extract(detail_json,'$.play_ids') FROM activity_events WHERE kind='bulk_watch'`).Scan(&bulkPlayIDs); err != nil {
		t.Fatal(err)
	}
	var mappedPlayCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity_events, json_each(activity_events.detail_json,'$.play_ids') AS play_ids WHERE activity_events.kind='bulk_watch'`).Scan(&mappedPlayCount); err != nil {
		t.Fatal(err)
	}
	if bulkPlayIDs != "[\"1\",\"2\"]" || mappedPlayCount != 2 {
		t.Fatalf("bulk activity play IDs were not remapped: %s (count %d)", bulkPlayIDs, mappedPlayCount)
	}
	var libraryID, libraryUserID string
	if err := db.QueryRow(`SELECT id,CAST(user_id AS TEXT) FROM user_media WHERE media_id='movie:77'`).Scan(&libraryID, &libraryUserID); err != nil {
		t.Fatal(err)
	}
	if libraryID != userID+":movie:77" || libraryUserID != userID {
		t.Fatalf("library relationship = (%q, %q), want (%q:movie:77, %q)", libraryID, libraryUserID, userID, userID)
	}
	var violations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
		t.Fatal(err)
	}
	if violations != 0 {
		t.Fatalf("migration left %d foreign key violations", violations)
	}
	userReferenceTables := []string{
		"user_settings", "sessions", "user_media", "plays", "activity_events", "episode_ratings",
		"personal_api_tokens", "notification_deliveries", "oauth_authorization_codes", "oauth_access_tokens",
		"oauth_refresh_tokens", "plex_webhooks", "plex_webhook_events",
	}
	for _, table := range userReferenceTables {
		var count int
		query := `SELECT COUNT(*) FROM ` + table + ` WHERE CAST(user_id AS TEXT)=?`
		if err := db.QueryRow(query, userID).Scan(&count); err != nil {
			t.Fatalf("check migrated user references in %s: %v", table, err)
		}
		if count == 0 {
			t.Errorf("legacy user reference missing from %s", table)
		}
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
