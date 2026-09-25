package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestPushoverSettings_GivenTwoUsers_WhenSavingAndClearingCredentials_ThenCredentialsStayPerUser(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user1ID := insertTestUser(t, store.DB, "user-1", "User 1", "private")
	user2ID := insertTestUser(t, store.DB, "user-2", "User 2", "private")
	app1, key1 := "encrypted-app-1", "encrypted-user-1"
	app2, key2 := "encrypted-app-2", "encrypted-user-2"
	if err := store.SetPushoverSettings(ctx, user1ID, &app1, &key1, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPushoverSettings(ctx, user2ID, &app2, &key2, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ id, app, key string }{{user1ID, app1, key1}, {user2ID, app2, key2}} {
		var app, key string
		if err := store.DB.QueryRowContext(ctx, `SELECT pushover_app_token_encrypted,pushover_user_key_encrypted FROM user_settings WHERE user_id=?`, want.id).Scan(&app, &key); err != nil {
			t.Fatal(err)
		}
		if app != want.app || key != want.key {
			t.Fatalf("credentials for %s = (%q, %q), want (%q, %q)", want.id, app, key, want.app, want.key)
		}
	}
	if err := store.ClearPushoverCredentials(ctx, user1ID); err != nil {
		t.Fatal(err)
	}
	var enabled, hasApp, hasKey bool
	if enabled, hasApp, hasKey, err = store.GetPushoverSettings(ctx, user1ID); err != nil || enabled || hasApp || hasKey {
		t.Fatalf("cleared user's settings = (%v, %v, %v), %v", enabled, hasApp, hasKey, err)
	}
	if enabled, hasApp, hasKey, err = store.GetPushoverSettings(ctx, user2ID); err != nil || !enabled || !hasApp || !hasKey {
		t.Fatalf("other user's settings = (%v, %v, %v), %v", enabled, hasApp, hasKey, err)
	}
}
