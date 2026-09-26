package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestWebPushCandidatesStaySeparateFromPushoverAndOlderEpisodes(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user := insertTestUser(t, store.DB, "push-user", "Alex", "private")
	_, err = store.DB.Exec(`UPDATE user_settings SET pushover_user_key_encrypted='user',pushover_app_token_encrypted='app',pushover_notifications_enabled=1 WHERE user_id=?`, user)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Show','2026-09-01','2026-09-01');
 INSERT INTO seasons(id,show_id,season_number) VALUES('season','tv:42',1),('specials','tv:42',0);
 INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,air_date) VALUES('older','tv:42','season',1,1,'2026-09-23'),('new','tv:42','season',1,2,'2026-09-26'),('special','tv:42','specials',0,1,'2026-09-26');`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at,notifications_since) VALUES(?,?,'tv:42','watching','2026-09-01','2026-09-01','2026-09-01')`, user+":tv:42", user)
	if err != nil {
		t.Fatal(err)
	}
	device := sqlite.PushSubscriptionID("https://push.example/device")
	if err := store.SavePushSubscription(ctx, user, device, "encrypted"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	// Pin registration time to the test's date.
	_, err = store.DB.Exec(`UPDATE web_push_subscriptions SET created_at='2026-09-26T00:00:00Z' WHERE id=?`, device)
	if err != nil {
		t.Fatal(err)
	}
	web, err := store.WebPushCandidates(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(web) != 1 || web[0].EpisodeID != "new" {
		t.Fatalf("web candidates=%+v", web)
	}
	if ok, err := store.ClaimWebPush(ctx, user, "new", device, now); err != nil || !ok {
		t.Fatalf("claim=%t err=%v", ok, err)
	}
	if err := store.CompleteWebPush(ctx, user, "new", device, now); err != nil {
		t.Fatal(err)
	}
	web, err = store.WebPushCandidates(ctx, now, 10)
	if err != nil || len(web) != 0 {
		t.Fatalf("web after send=%+v err=%v", web, err)
	}
	pushover, err := store.NotificationCandidates(ctx, now, 10)
	if err != nil || len(pushover) != 2 {
		t.Fatalf("pushover candidates=%+v err=%v", pushover, err)
	}
}
