package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestNotificationCandidates_GivenOptedInWatchingShow_WhenEpisodeHasAired_ThenOnlyUnwatchedRegularEpisodeIsReturned(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "user-1", "Alex", "private")
	_, err = store.DB.Exec(`UPDATE user_settings SET pushover_user_key_encrypted='user-ciphertext',pushover_notifications_enabled=1,pushover_app_token_encrypted='app-ciphertext' WHERE user_id=?`, userID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example Show','2026-09-01','2026-09-01');
		INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season 1'),('tv:42:season:0','tv:42',0,'Specials');
		INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name,air_date) VALUES
		('episode-watched','tv:42','tv:42:season:1',1,1,'Watched Episode','2026-09-20'),
		('episode-new','tv:42','tv:42:season:1',1,2,'New Episode','2026-09-23'),
		('episode-future','tv:42','tv:42:season:1',1,3,'Future Episode','2026-10-01'),
		('episode-special','tv:42','tv:42:season:0',0,1,'Special','2026-09-23');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at,notifications_since) VALUES(?,?,'tv:42','watching','2026-09-01','2026-09-01','2026-09-01')`, userID+":tv:42", userID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES(?,'episode-watched','2026-09-22','web','2026-09-22')`, userID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	candidates, err := store.NotificationCandidates(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].EpisodeID != "episode-new" || candidates[0].ShowTitle != "Example Show" || candidates[0].EncryptedAppToken != "app-ciphertext" || candidates[0].EncryptedUserKey != "user-ciphertext" {
		t.Fatalf("notification candidates=%+v", candidates)
	}
	claimedAt := now
	claimed, err := store.ClaimNotification(ctx, userID, "episode-new", claimedAt)
	if err != nil || !claimed {
		t.Fatalf("first claim = %v, %v", claimed, err)
	}
	claimed, err = store.ClaimNotification(ctx, userID, "episode-new", claimedAt)
	if err != nil || claimed {
		t.Fatalf("second claim = %v, %v; want duplicate denied", claimed, err)
	}
	if err := store.CompleteNotification(ctx, userID, "episode-new", claimedAt); err != nil {
		t.Fatal(err)
	}
	candidates, err = store.NotificationCandidates(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("sent episode was returned again: %+v", candidates)
	}
}

func TestNotificationCandidates_GivenFailedDelivery_WhenBackoffExpires_ThenItRetriesAtMostThreeTimes(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "user-1", "Alex", "private")
	_, err = store.DB.Exec(`UPDATE user_settings SET pushover_user_key_encrypted='user-ciphertext',pushover_notifications_enabled=1,pushover_app_token_encrypted='app-ciphertext' WHERE user_id=?`, userID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example Show','2026-09-01','2026-09-01');
		INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season 1');
		INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name,air_date) VALUES('episode-new','tv:42','tv:42:season:1',1,1,'New Episode','2026-09-23');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at,notifications_since) VALUES(?,?,'tv:42','watching','2026-09-01','2026-09-01','2026-09-01')`, userID+":tv:42", userID); err != nil {
		t.Fatal(err)
	}
	firstAttempt := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if claimed, err := store.ClaimNotification(ctx, userID, "episode-new", firstAttempt); err != nil || !claimed {
		t.Fatalf("first claim=%v err=%v", claimed, err)
	}
	if err := store.FailNotification(ctx, userID, "episode-new", firstAttempt); err != nil {
		t.Fatal(err)
	}
	checkCandidates := func(at time.Time, want int) {
		t.Helper()
		candidates, err := store.NotificationCandidates(ctx, at, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) != want {
			t.Fatalf("at %s candidates=%+v, want %d", at, candidates, want)
		}
	}
	checkCandidates(firstAttempt.Add(14*time.Minute), 0)
	secondAttempt := firstAttempt.Add(15 * time.Minute)
	checkCandidates(secondAttempt, 1)
	if claimed, err := store.ClaimNotification(ctx, userID, "episode-new", secondAttempt); err != nil || !claimed {
		t.Fatalf("second claim=%v err=%v", claimed, err)
	}
	if err := store.FailNotification(ctx, userID, "episode-new", secondAttempt); err != nil {
		t.Fatal(err)
	}
	thirdAttempt := secondAttempt.Add(30 * time.Minute)
	checkCandidates(thirdAttempt.Add(-time.Second), 0)
	checkCandidates(thirdAttempt, 1)
	if claimed, err := store.ClaimNotification(ctx, userID, "episode-new", thirdAttempt); err != nil || !claimed {
		t.Fatalf("third claim=%v err=%v", claimed, err)
	}
	if err := store.FailNotification(ctx, userID, "episode-new", thirdAttempt); err != nil {
		t.Fatal(err)
	}
	checkCandidates(thirdAttempt.Add(24*time.Hour), 0)
}
