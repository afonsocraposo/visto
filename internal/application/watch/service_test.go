package watch

import (
	"context"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

type repository struct {
	shows    []Show
	timezone string
}

func (r repository) WatchingShows(context.Context, string) ([]Show, error) { return r.shows, nil }
func (r repository) Timezone(context.Context, string) (string, error)      { return r.timezone, nil }

func TestContinue_GivenLaterEpisodeWatchedAndEarlierEpisodeMissing_WhenLoadingNow_ThenItSuggestsTheEpisodeAfterFurthestWatched(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	air1, air2, air3 := now.AddDate(0, 0, -5), now.AddDate(0, 0, -4), now.AddDate(0, 0, -3)
	show := Show{ID: "tv:1", Title: "The Example", Episodes: []domain.Episode{{ID: "e1", SeasonNumber: 1, EpisodeNumber: 1, AirDate: &air1}, {ID: "e2", SeasonNumber: 1, EpisodeNumber: 2, AirDate: &air2}, {ID: "e3", SeasonNumber: 1, EpisodeNumber: 3, AirDate: &air3}, {ID: "e4", SeasonNumber: 1, EpisodeNumber: 4, AirDate: &now}}, Plays: []domain.EpisodePlay{{ID: "p3", EpisodeID: "e3"}}}
	service := NewService(repository{shows: []Show{show}, timezone: "UTC"})
	service.now = func() time.Time { return now }
	entries, err := service.Continue(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].NextEpisode == nil || entries[0].NextEpisode.ID != "e4" {
		t.Fatalf("entries=%+v", entries)
	}
	if len(entries[0].MissingPriorEpisodes) != 2 || entries[0].MissingPriorEpisodes[0].ID != "e1" || entries[0].MissingPriorEpisodes[1].ID != "e2" {
		t.Fatalf("missing prior=%+v", entries[0].MissingPriorEpisodes)
	}
}

func TestCalendar_GivenFutureEpisodeWatchedBeforeAirDate_WhenListingUpcoming_ThenItIsOmitted(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 30)
	watched := from.AddDate(0, 0, 3)
	pending := from.AddDate(0, 0, 5)
	show := Show{ID: "tv:1", Title: "The Example", Episodes: []domain.Episode{{ID: "leak", SeasonNumber: 2, EpisodeNumber: 1, AirDate: &watched}, {ID: "upcoming", SeasonNumber: 2, EpisodeNumber: 2, AirDate: &pending}}, Plays: []domain.EpisodePlay{{ID: "play", EpisodeID: "leak"}}}
	service := NewService(repository{shows: []Show{show}, timezone: "UTC"})
	service.now = func() time.Time { return from }
	entries, err := service.Calendar(context.Background(), "user", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Episode.ID != "upcoming" {
		t.Fatalf("entries=%+v", entries)
	}
}

func TestContinue_GivenNoReleasedEpisodeAfterProgress_WhenLoadingNow_ThenCaughtUpShowIsOmitted(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 0, 14)
	show := Show{ID: "tv:1", Title: "The Example", Episodes: []domain.Episode{{ID: "watched", SeasonNumber: 1, EpisodeNumber: 1, AirDate: &now}, {ID: "future", SeasonNumber: 1, EpisodeNumber: 2, AirDate: &future}}, Plays: []domain.EpisodePlay{{ID: "play", EpisodeID: "watched"}}}
	service := NewService(repository{shows: []Show{show}, timezone: "UTC"})
	service.now = func() time.Time { return now }
	entries, err := service.Continue(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries=%+v, want caught-up show omitted", entries)
	}
}
