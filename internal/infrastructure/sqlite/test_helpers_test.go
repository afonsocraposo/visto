package sqlite_test

import (
	"database/sql"
	"strconv"
	"testing"
)

func insertTestUser(t *testing.T, db *sql.DB, username, displayName, visibility string) string {
	t.Helper()
	result, err := db.Exec(`INSERT INTO users(username,display_name,password_hash,role,created_at,updated_at,email)
		VALUES(?,?,'hash','user',?,?,?)`, username, displayName, testTimestamp, testTimestamp, username+"@example.test")
	if err != nil {
		t.Fatalf("insert test user %q: %v", username, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("get test user ID for %q: %v", username, err)
	}
	userID := strconv.FormatInt(id, 10)
	if _, err := db.Exec(`INSERT INTO user_settings(user_id,timezone,activity_visibility,created_at,updated_at)
		VALUES(?,'UTC',?,?,?)`, userID, visibility, testTimestamp, testTimestamp); err != nil {
		t.Fatalf("insert settings for test user %q: %v", username, err)
	}
	return userID
}
