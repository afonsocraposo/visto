package sqlite_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/notifications"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

type releaseNotificationSender struct{ sent int }

func (s *releaseNotificationSender) Send(_ context.Context, _, _, title, body string) error {
	s.sent++
	if title != "Now available: Example Movie" || body != "Example Movie is out today." {
		return fmt.Errorf("unexpected movie release notification: %q / %q", title, body)
	}
	return nil
}

type releaseNotificationDecryptor struct{}

func (releaseNotificationDecryptor) Decrypt(value string) (string, error) { return value, nil }

type seasonNotificationSender struct{ bodies []string }

func (s *seasonNotificationSender) Send(_ context.Context, _, _, _, body string) error {
	s.bodies = append(s.bodies, body)
	return nil
}

func TestMovieReleaseNotification_GivenWatchlistedUnreleasedMovie_WhenReleaseDateBecomesToday_ThenItSendsOnce(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "movie-alert-user", "Alex", "private")
	if _, err := store.DB.Exec(`UPDATE user_settings SET pushover_user_key_encrypted='user-key',pushover_app_token_encrypted='app-token',pushover_notifications_enabled=1 WHERE user_id=?`, userID); err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,release_date,metadata_updated_at,created_at)
		VALUES('movie:42','movie',42,'Example Movie','2026-10-02','2026-09-01','2026-09-01');
		INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at,notifications_enabled)
		VALUES(?,?,'movie:42','watchlist','2026-09-01','2026-09-01',0);`, userID+":movie:42", userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := library.NewService(store).SetNotificationsEnabled(ctx, userID, "movie:42", true); err != nil {
		t.Fatalf("enable movie release alert: %v", err)
	}
	sender := &releaseNotificationSender{}
	service := &notifications.MovieReleaseService{Repository: store, Pushover: sender, Decryptor: releaseNotificationDecryptor{}}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return now }
	if err := service.Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.sent != 0 {
		t.Fatalf("future movie sent %d alerts", sender.sent)
	}
	if _, err := store.DB.Exec(`UPDATE media SET release_date='2026-10-01' WHERE id='movie:42'`); err != nil {
		t.Fatal(err)
	}
	now = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for range 2 {
		if err := service.Dispatch(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if sender.sent != 1 {
		t.Fatalf("sent alerts = %d, want exactly one", sender.sent)
	}
	var deliveries int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM movie_release_deliveries WHERE user_id=? AND media_id='movie:42' AND channel='pushover' AND state='sent'`, userID).Scan(&deliveries); err != nil || deliveries != 1 {
		t.Fatalf("sent deliveries=%d err=%v, want 1", deliveries, err)
	}
}

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
	if _, err := store.DB.Exec(`UPDATE user_media SET status='dropped' WHERE user_id=? AND media_id='tv:42'`, userID); err != nil {
		t.Fatal(err)
	}
	candidates, err = store.NotificationCandidates(ctx, now, 10)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("non-Watching show produced notification candidates=%+v err=%v", candidates, err)
	}
	if _, err := store.DB.Exec(`UPDATE user_media SET status='watching' WHERE user_id=? AND media_id='tv:42'`, userID); err != nil {
		t.Fatal(err)
	}
	candidates, err = store.NotificationCandidates(ctx, now, 10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("Watching show candidates after restore=%+v err=%v", candidates, err)
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

func TestShowNotificationMode_GivenSeasonChoice_WhenNewSeasonsBecomeReady_ThenEachSeasonAlertsOnce(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "season-mode-user", "Alex", "private")
	if _, err := store.DB.Exec(`UPDATE user_settings SET pushover_user_key_encrypted='user-key',pushover_app_token_encrypted='app-token',pushover_notifications_enabled=1 WHERE user_id=?`, userID); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(24 * time.Hour)
	firstAirDate := start.AddDate(0, 0, 1).Format("2006-01-02")
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example Show','now','now');
		INSERT INTO seasons(id,show_id,season_number,name,episode_count) VALUES('season-1','tv:42',1,'Season 1',1)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,air_date) VALUES('episode-1','tv:42','season-1',1,1,?)`, firstAirDate); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','watching',?,?)`, userID+":tv:42", userID, start.Format(time.RFC3339Nano), start.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := library.NewService(store).SetShowNotificationMode(ctx, userID, "tv:42", "season"); err != nil {
		t.Fatal(err)
	}
	entry, err := store.GetMediaByTMDBID(ctx, userID, "tv", 42)
	if err != nil || !entry.Item.SeasonAlertsEnabled || entry.Item.NotificationsEnabled {
		t.Fatalf("season choice = %+v, err=%v", entry.Item, err)
	}
	if candidates, err := store.NotificationCandidates(ctx, start.AddDate(0, 0, 2), 10); err != nil || len(candidates) != 0 {
		t.Fatalf("episode alerts in season mode = %+v, err=%v", candidates, err)
	}
	sender := &seasonNotificationSender{}
	now := start.AddDate(0, 0, 2)
	service := &notifications.SeasonService{Repository: store, Pushover: sender, Decryptor: releaseNotificationDecryptor{}, Now: func() time.Time { return now }}
	if err := service.Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	secondAirDate := start.AddDate(0, 0, 3).Format("2006-01-02")
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name,episode_count) VALUES('season-2','tv:42',2,'Season 2',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,air_date) VALUES('episode-2','tv:42','season-2',2,1,?)`, secondAirDate); err != nil {
		t.Fatal(err)
	}
	now = start.AddDate(0, 0, 4)
	for range 2 {
		if err := service.Dispatch(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(sender.bodies) != 2 || sender.bodies[0] != "All episodes of Example Show season 1 are available." || sender.bodies[1] != "All episodes of Example Show season 2 are available." {
		t.Fatalf("season notifications = %#v, want one for each season", sender.bodies)
	}
	var ready int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM season_ready_alerts WHERE user_id=? AND ready_at IS NOT NULL`, userID).Scan(&ready); err != nil || ready != 2 {
		t.Fatalf("ready season alerts=%d, err=%v; want 2", ready, err)
	}
	thirdAirDate := start.AddDate(0, 0, 5).Format("2006-01-02")
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name,episode_count) VALUES('season-3','tv:42',3,'Season 3',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,air_date) VALUES('episode-3','tv:42','season-3',3,1,?)`, thirdAirDate); err != nil {
		t.Fatal(err)
	}
	if err := service.Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if err := library.NewService(store).SetShowNotificationMode(ctx, userID, "tv:42", "episode"); err != nil {
		t.Fatal(err)
	}
	entry, err = store.GetMediaByTMDBID(ctx, userID, "tv", 42)
	if err != nil || entry.Item.SeasonAlertsEnabled || !entry.Item.NotificationsEnabled {
		t.Fatalf("episode choice = %+v, err=%v", entry.Item, err)
	}
	now = start.AddDate(0, 0, 6)
	if err := service.Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sender.bodies) != 2 {
		t.Fatalf("season alerts after choosing episodes = %#v", sender.bodies)
	}
	candidates, err := store.NotificationCandidates(ctx, now, 10)
	if err != nil || len(candidates) != 1 || candidates[0].EpisodeID != "episode-3" {
		t.Fatalf("episode alerts after switching modes = %+v, err=%v", candidates, err)
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
