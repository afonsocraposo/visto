package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	exportapp "github.com/afonsocosta/visto/internal/application/export"
	"github.com/afonsocosta/visto/internal/application/feed"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/profile"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestVistoWorkflow_GivenTwoUsersAndOneSharedShow_WhenTheyTrackAndExport_ThenProgressActivityAndDataStayUserScoped(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	accounts := auth.NewService(store)
	alice, err := accounts.Bootstrap(ctx, "afonso@example.com", "Afonso", "a-long-admin-password")
	if err != nil {
		t.Fatalf("bootstrap first admin: %v", err)
	}
	bia, err := accounts.CreateUser(ctx, "bia@example.com", "Bia", "a-long-family-password")
	if err != nil {
		t.Fatalf("create family user: %v", err)
	}
	profiles := profile.NewService(store)
	if err := profiles.Update(ctx, alice.ID, profile.Settings{ActivityVisibility: profile.InstanceVisibility, Timezone: "Europe/Lisbon"}); err != nil {
		t.Fatalf("opt Afonso into the instance feed: %v", err)
	}
	if err := profiles.Update(ctx, bia.ID, profile.Settings{ActivityVisibility: profile.PrivateVisibility, Timezone: "UTC"}); err != nil {
		t.Fatalf("keep Bia's activity private: %v", err)
	}

	libraries := library.NewService(store)
	show := library.Media{Type: domain.TVMediaType, TMDBID: 42, Title: "Example Show"}
	aliceRating, biaRating := 5, 3
	if _, err := libraries.SaveMedia(ctx, alice.ID, show, domain.WatchingStatus, &aliceRating); err != nil {
		t.Fatalf("add show for Afonso: %v", err)
	}
	if _, err := libraries.SaveMedia(ctx, bia.ID, show, domain.WatchingStatus, &biaRating); err != nil {
		t.Fatalf("add show for Bia: %v", err)
	}
	if err := store.ImportShowMetadata(ctx, "tv:42", domain.TVShowMetadata{
		TMDBID: 42,
		Name:   "Example Show",
		Seasons: []domain.TVSeasonMetadata{
			{TMDBID: 410, Number: 1, Name: "Season 1", Episodes: []domain.TVEpisodeMetadata{
				{TMDBID: 4101, SeasonNumber: 1, EpisodeNumber: 1, Name: "Pilot", AirDate: "2020-01-01"},
				{TMDBID: 4102, SeasonNumber: 1, EpisodeNumber: 2, Name: "Second", AirDate: "2020-01-02"},
			}},
			{TMDBID: 550, Number: 15, Name: "Season 15", Episodes: []domain.TVEpisodeMetadata{
				{TMDBID: 5501, SeasonNumber: 15, EpisodeNumber: 1, Name: "Premiere", AirDate: "2020-02-01"},
				{TMDBID: 5502, SeasonNumber: 15, EpisodeNumber: 2, Name: "Second", AirDate: "2020-02-02"},
			}},
		},
	}); err != nil {
		t.Fatalf("import show catalog: %v", err)
	}

	trackingService := tracking.NewService(store)
	aliceEpisode, biaEpisode := "tv:42:episode:5501", "tv:42:episode:4101"
	watchedAt := time.Date(2020, 3, 1, 12, 0, 0, 0, time.UTC)
	alicePlay, err := trackingService.Record(ctx, alice.ID, nil, &aliceEpisode, watchedAt, "web")
	if err != nil {
		t.Fatalf("record Afonso's season 15 play: %v", err)
	}
	biaPlay, err := trackingService.Record(ctx, bia.ID, nil, &biaEpisode, watchedAt, "web")
	if err != nil {
		t.Fatalf("record Bia's season 1 play: %v", err)
	}

	watching := watch.NewService(store)
	for _, expectation := range []struct {
		userID, nextEpisodeID string
		missingEpisodeIDs     []string
	}{
		{alice.ID, "tv:42:episode:5502", []string{"tv:42:episode:4101", "tv:42:episode:4102"}},
		{bia.ID, "tv:42:episode:4102", nil},
	} {
		entries, err := watching.Continue(ctx, expectation.userID)
		if err != nil {
			t.Fatalf("load Continue Watching for %s: %v", expectation.userID, err)
		}
		if len(entries) != 1 || entries[0].NextEpisode == nil || entries[0].NextEpisode.ID != expectation.nextEpisodeID {
			t.Fatalf("continue entries for %s=%+v, want next episode %s", expectation.userID, entries, expectation.nextEpisodeID)
		}
		if len(entries[0].MissingPriorEpisodes) != len(expectation.missingEpisodeIDs) {
			t.Fatalf("missing episodes for %s=%+v, want %v", expectation.userID, entries[0].MissingPriorEpisodes, expectation.missingEpisodeIDs)
		}
		for index, episodeID := range expectation.missingEpisodeIDs {
			if entries[0].MissingPriorEpisodes[index].ID != episodeID {
				t.Fatalf("missing episodes for %s=%+v, want %v", expectation.userID, entries[0].MissingPriorEpisodes, expectation.missingEpisodeIDs)
			}
		}
	}

	page, err := feed.NewService(store).List(ctx, "", 100)
	if err != nil {
		t.Fatalf("load instance feed: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("feed items=%+v, want Afonso's opted-in rating and watch only", page.Items)
	}
	feedKinds := map[string]int{}
	for _, item := range page.Items {
		if item.DisplayName != alice.DisplayName {
			t.Fatalf("private user's activity leaked into feed: %+v", item)
		}
		feedKinds[item.Kind]++
		if item.Kind == "rating" && (item.Rating == nil || *item.Rating != aliceRating) {
			t.Fatalf("feed rating=%+v, want Afonso's five-star rating", item)
		}
	}
	if feedKinds["rating"] != 1 || feedKinds["watch"] != 1 {
		t.Fatalf("feed kinds=%v, want one rating and one watch", feedKinds)
	}

	exports := exportapp.NewService(store)
	for _, expectation := range []struct {
		userID, playID string
		rating         int
	}{
		{alice.ID, alicePlay.ID, aliceRating},
		{bia.ID, biaPlay.ID, biaRating},
	} {
		history, err := trackingService.History(ctx, expectation.userID, 100)
		if err != nil {
			t.Fatalf("load history for %s: %v", expectation.userID, err)
		}
		if len(history) != 1 || history[0].Play.ID != expectation.playID {
			t.Fatalf("history for %s=%+v, want only play %s", expectation.userID, history, expectation.playID)
		}
		libraryEntries, err := libraries.List(ctx, expectation.userID)
		if err != nil {
			t.Fatalf("load library for %s: %v", expectation.userID, err)
		}
		if len(libraryEntries) != 1 || libraryEntries[0].Item.Rating == nil || *libraryEntries[0].Item.Rating != expectation.rating {
			t.Fatalf("library for %s=%+v, want only that user's %d-star rating", expectation.userID, libraryEntries, expectation.rating)
		}
		data, err := exports.Data(ctx, expectation.userID)
		if err != nil {
			t.Fatalf("export data for %s: %v", expectation.userID, err)
		}
		if len(data.Library) != 1 || data.Library[0].MediaID != "tv:42" || data.Library[0].Rating == nil || *data.Library[0].Rating != expectation.rating || len(data.Plays) != 1 || data.Plays[0].ID != expectation.playID {
			t.Fatalf("export for %s=%+v, want one shared show and only that user's play", expectation.userID, data)
		}
	}
}
