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
	_, err = store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES('user-1','user-1','Alex','hash','user','2026-09-01','2026-09-01');
		INSERT INTO user_settings(user_id,timezone,activity_visibility,created_at,updated_at,pushover_user_key_encrypted,pushover_notifications_enabled) VALUES('user-1','UTC','private','2026-09-01','2026-09-01','ciphertext',1);
		INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example Show','2026-09-01','2026-09-01');
		INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season 1'),('tv:42:season:0','tv:42',0,'Specials');
		INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name,air_date) VALUES
		('episode-watched','tv:42','tv:42:season:1',1,1,'Watched Episode','2026-09-20'),
		('episode-new','tv:42','tv:42:season:1',1,2,'New Episode','2026-09-23'),
		('episode-future','tv:42','tv:42:season:1',1,3,'Future Episode','2026-10-01'),
		('episode-special','tv:42','tv:42:season:0',0,1,'Special','2026-09-23');
		INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at,notifications_since) VALUES('user-1:tv:42','user-1','tv:42','watching','2026-09-01','2026-09-01','2026-09-01');
		INSERT INTO plays(id,user_id,episode_id,watched_at,source,created_at) VALUES('play-1','user-1','episode-watched','2026-09-22','web','2026-09-22');`)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := store.NotificationCandidates(ctx, "2026-09-24", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].EpisodeID != "episode-new" || candidates[0].ShowTitle != "Example Show" {
		t.Fatalf("notification candidates=%+v", candidates)
	}
	claimedAt := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	claimed, err := store.ClaimNotification(ctx, "user-1", "episode-new", claimedAt)
	if err != nil || !claimed {
		t.Fatalf("first claim = %v, %v", claimed, err)
	}
	claimed, err = store.ClaimNotification(ctx, "user-1", "episode-new", claimedAt)
	if err != nil || claimed {
		t.Fatalf("second claim = %v, %v; want duplicate denied", claimed, err)
	}
	if err := store.CompleteNotification(ctx, "user-1", "episode-new", claimedAt); err != nil {
		t.Fatal(err)
	}
	candidates, err = store.NotificationCandidates(ctx, "2026-09-24", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("sent episode was returned again: %+v", candidates)
	}
}
