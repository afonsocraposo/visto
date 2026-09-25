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
	_, err = store.DB.Exec(`
		INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES
		('user-1','user-1','User 1','hash','user','2026-09-01','2026-09-01'),
		('user-2','user-2','User 2','hash','user','2026-09-01','2026-09-01');
		INSERT INTO user_settings(user_id,timezone,activity_visibility,created_at,updated_at) VALUES
		('user-1','UTC','private','2026-09-01','2026-09-01'),
		('user-2','UTC','private','2026-09-01','2026-09-01');`)
	if err != nil {
		t.Fatal(err)
	}
	app1, key1 := "encrypted-app-1", "encrypted-user-1"
	app2, key2 := "encrypted-app-2", "encrypted-user-2"
	if err := store.SetPushoverSettings(ctx, "user-1", &app1, &key1, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPushoverSettings(ctx, "user-2", &app2, &key2, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ id, app, key string }{{"user-1", app1, key1}, {"user-2", app2, key2}} {
		var app, key string
		if err := store.DB.QueryRowContext(ctx, `SELECT pushover_app_token_encrypted,pushover_user_key_encrypted FROM user_settings WHERE user_id=?`, want.id).Scan(&app, &key); err != nil {
			t.Fatal(err)
		}
		if app != want.app || key != want.key {
			t.Fatalf("credentials for %s = (%q, %q), want (%q, %q)", want.id, app, key, want.app, want.key)
		}
	}
	if err := store.ClearPushoverCredentials(ctx, "user-1"); err != nil {
		t.Fatal(err)
	}
	var enabled, hasApp, hasKey bool
	if enabled, hasApp, hasKey, err = store.GetPushoverSettings(ctx, "user-1"); err != nil || enabled || hasApp || hasKey {
		t.Fatalf("cleared user's settings = (%v, %v, %v), %v", enabled, hasApp, hasKey, err)
	}
	if enabled, hasApp, hasKey, err = store.GetPushoverSettings(ctx, "user-2"); err != nil || !enabled || !hasApp || !hasKey {
		t.Fatalf("other user's settings = (%v, %v, %v), %v", enabled, hasApp, hasKey, err)
	}
}
