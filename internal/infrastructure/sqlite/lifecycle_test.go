package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestShowLifecycle_CompletesAndReopensFromEpisodeHistory(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "lifecycle", "Lifecycle", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,status,metadata_updated_at,created_at)
		VALUES('tv:42','tv',42,'Example','Ended',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at)
		VALUES(?,?,'tv:42','watchlist',?,?)`, userID+":tv:42", userID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteMedia(ctx, userID, "tv:42", nil, "web"); err == nil {
		t.Fatal("incomplete catalog was completed")
	}
	for _, next := range []domain.LibraryStatus{domain.PausedStatus, domain.WatchlistStatus} {
		if err := store.UpsertItem(ctx, library.Item{UserID: userID, MediaID: "tv:42", Status: next, AddedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	show := domain.TVShowMetadata{TMDBID: 42, Name: "Example", Status: "Ended", Seasons: []domain.TVSeasonMetadata{
		{Number: 0, Name: "Specials", Episodes: []domain.TVEpisodeMetadata{{TMDBID: 4200, SeasonNumber: 0, EpisodeNumber: 1, Name: "Extra"}}},
		{Number: 1, Name: "Season 1", EpisodeCount: 2, Episodes: []domain.TVEpisodeMetadata{
			{TMDBID: 4201, SeasonNumber: 1, EpisodeNumber: 1, Name: "Pilot"},
			{TMDBID: 4202, SeasonNumber: 1, EpisodeNumber: 2, Name: "Finale", AirDate: "2030-01-01"},
		}},
	}}
	partial := show
	partial.Seasons = append([]domain.TVSeasonMetadata(nil), show.Seasons...)
	partial.Seasons[1].Episodes = partial.Seasons[1].Episodes[:1]
	if err := store.ImportShowMetadata(ctx, "tv:42", partial); err != nil {
		t.Fatal(err)
	}
	status := func() string {
		t.Helper()
		var value string
		if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id='tv:42'`, userID).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := "tv:42:episode:4201"
	if _, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, EpisodeID: &first, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != "watching" {
		t.Fatalf("after first episode: %s", got)
	}
	if _, err := store.CompleteMedia(ctx, userID, "tv:42", nil, "web"); err == nil {
		t.Fatal("partially imported show was completed")
	}
	if err := store.ImportShowMetadata(ctx, "tv:42", show); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertItem(ctx, library.Item{UserID: userID, MediaID: "tv:42", Status: domain.WatchlistStatus, AddedAt: time.Now(), UpdatedAt: time.Now()}); err == nil {
		t.Fatal("watched show returned to Watchlist")
	}
	if err := store.UpsertItem(ctx, library.Item{UserID: userID, MediaID: "tv:42", Status: domain.DroppedStatus, AddedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertItem(ctx, library.Item{UserID: userID, MediaID: "tv:42", Status: domain.CompletedStatus, AddedAt: time.Now(), UpdatedAt: time.Now()}); err == nil {
		t.Fatal("show completed without confirming and recording missing episodes")
	}
	second := "tv:42:episode:4202"
	if _, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, EpisodeID: &second, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != "completed" {
		t.Fatalf("after future finale: %s", got)
	}
	rating := 5
	if err := store.UpsertItem(ctx, library.Item{UserID: userID, MediaID: "tv:42", Status: domain.CompletedStatus, Rating: &rating, AddedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatalf("rate completed show: %v", err)
	}
	if err := store.RemoveStatusItem(ctx, userID, "tv:42", domain.CompletedStatus); err == nil {
		t.Fatal("completed show was removed without its history")
	}
	rewatch, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, EpisodeID: &second, WatchedAt: time.Now().UTC(), Source: "web"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePlay(ctx, userID, rewatch.ID); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != "completed" {
		t.Fatalf("after deleting only a rewatch: %s", got)
	}
	if err := store.DeleteEpisodePlays(ctx, userID, []string{second}); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != "watching" {
		t.Fatalf("after marking finale unwatched: %s", got)
	}
	completedItem, err := store.CompleteMedia(ctx, userID, "tv:42", nil, "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(completedItem.CreatedEpisodeIDs) != 1 || completedItem.CreatedEpisodeIDs[0] != second {
		t.Fatalf("completion created episodes=%v, want only %s", completedItem.CreatedEpisodeIDs, second)
	}
	if got := status(); got != "completed" {
		t.Fatalf("after manual completion: %s", got)
	}
	var specialPlays int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=? AND episode_id='tv:42:episode:4200'`, userID).Scan(&specialPlays); err != nil {
		t.Fatal(err)
	}
	if specialPlays != 0 {
		t.Fatal("manual completion marked a special watched")
	}
	show.Status = "Returning Series"
	if err := store.ImportShowMetadata(ctx, "tv:42", show); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != "watching" {
		t.Fatalf("after show becomes ongoing: %s", got)
	}
	show.Status = "Ended"
	if err := store.ImportShowMetadata(ctx, "tv:42", show); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != "completed" {
		t.Fatalf("after show becomes terminal again: %s", got)
	}
	show.Seasons[1].EpisodeCount = 3
	show.Seasons[1].Episodes = append(show.Seasons[1].Episodes, domain.TVEpisodeMetadata{TMDBID: 4203, SeasonNumber: 1, EpisodeNumber: 3, Name: "New Episode"})
	if err := store.ImportShowMetadata(ctx, "tv:42", show); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != "watching" {
		t.Fatalf("after catalog gains an episode: %s", got)
	}
}
