package watch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

type metadataRepository struct {
	repository
	showIDs      []int64
	imported     []int64
	refreshLimit int
}

func (r *metadataRepository) ShowsNeedingCatalogRefresh(_ context.Context, _, _ time.Duration, limit int) ([]int64, error) {
	r.refreshLimit = limit
	if len(r.showIDs) > limit {
		return r.showIDs[:limit], nil
	}
	return r.showIDs, nil
}
func (r *metadataRepository) ImportShowMetadata(_ context.Context, showID string, metadata domain.TVShowMetadata) error {
	if showID != fmt.Sprintf("tv:%d", metadata.TMDBID) {
		return fmt.Errorf("show ID %q does not match metadata ID %d", showID, metadata.TMDBID)
	}
	r.imported = append(r.imported, metadata.TMDBID)
	return nil
}

type showMetadataProvider struct {
	calls        []int64
	refreshCalls []int64
}

func (p *showMetadataProvider) Show(_ context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	p.calls = append(p.calls, tmdbID)
	return domain.TVShowMetadata{TMDBID: tmdbID, Name: fmt.Sprintf("Show %d", tmdbID)}, nil
}

func (p *showMetadataProvider) RefreshShow(_ context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	p.refreshCalls = append(p.refreshCalls, tmdbID)
	return domain.TVShowMetadata{TMDBID: tmdbID, Name: fmt.Sprintf("Show %d", tmdbID)}, nil
}

type repository struct {
	shows          []Show
	timezone       string
	episodes       []ShowEpisode
	seasons        []Season
	seasonEpisodes []ShowEpisode
}

func (r repository) WatchingShows(context.Context, string) ([]Show, error) { return r.shows, nil }
func (r repository) Timezone(context.Context, string) (string, error)      { return r.timezone, nil }
func (r repository) ListShowEpisodes(context.Context, string, string) ([]ShowEpisode, error) {
	return r.episodes, nil
}
func (r repository) ListShowSeasons(context.Context, string, string) ([]Season, error) {
	return r.seasons, nil
}
func (r repository) ListSeasonEpisodes(context.Context, string, string) ([]ShowEpisode, error) {
	return r.seasonEpisodes, nil
}

func TestShowProgress_GivenASeasonFifteenPlay_WhenProgressIsRequested_ThenTheCursorAndNextEpisodeUseTheFurthestPlay(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	date := func(day int) *time.Time {
		value := now.AddDate(0, 0, day)
		return &value
	}
	entries := []ShowEpisode{
		{Episode: domain.Episode{ID: "s1e1", SeasonNumber: 1, EpisodeNumber: 1, AirDate: date(-20)}},
		{Episode: domain.Episode{ID: "s15e20", SeasonNumber: 15, EpisodeNumber: 20, AirDate: date(-2)}},
		{Episode: domain.Episode{ID: "s16e1", SeasonNumber: 16, EpisodeNumber: 1, AirDate: date(-1)}},
		{Episode: domain.Episode{ID: "s16e2", SeasonNumber: 16, EpisodeNumber: 2, AirDate: date(20)}},
	}
	entries[1].Watched = true
	service := NewService(repository{timezone: "UTC", episodes: entries})
	service.now = func() time.Time { return now }

	progress, err := service.ShowProgress(context.Background(), "user-1", "tv:example")
	if err != nil {
		t.Fatalf("get show progress: %v", err)
	}
	if progress.Cursor == nil || progress.Cursor.ID != "s15e20" {
		t.Fatalf("cursor=%+v, want s15e20", progress.Cursor)
	}
	if progress.NextEpisode == nil || progress.NextEpisode.ID != "s16e1" || progress.IsCaughtUp {
		t.Fatalf("progress=%+v, want next episode s16e1 and not caught up", progress)
	}
}

func TestEpisodes_GivenShowInUsersLibrary_WhenEpisodesAreRequested_ThenTheCatalogAndWatchedStateAreReturned(t *testing.T) {
	episodes := []ShowEpisode{
		{Episode: domain.Episode{ID: "s1e1", SeasonNumber: 1, EpisodeNumber: 1}, Name: "Pilot", Watched: true},
		{Episode: domain.Episode{ID: "s15e1", SeasonNumber: 15, EpisodeNumber: 1}, Name: "Premiere"},
	}
	service := NewService(repository{episodes: episodes})
	result, err := service.Episodes(context.Background(), "user", "tv:example")
	if err != nil {
		t.Fatalf("list show episodes: %v", err)
	}
	if len(result) != 2 || result[0].Episode.SeasonNumber != 1 || result[1].Episode.SeasonNumber != 15 {
		t.Fatalf("episodes=%+v, want the season 1 and season 15 episodes", result)
	}
	if !result[0].Watched || result[1].Watched {
		t.Fatalf("watched states=%t,%t, want true,false", result[0].Watched, result[1].Watched)
	}
}

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

func TestContinue_GivenEpisodeArtworkAndName_WhenLoadingWatching_ThenItIncludesBoth(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	show := Show{
		ID: "tv:1", Title: "The Example",
		Episodes:       []domain.Episode{{ID: "e1", SeasonNumber: 1, EpisodeNumber: 1, AirDate: &now}},
		EpisodeDetails: map[string]EpisodeDisplay{"e1": {Name: "Pilot", StillPath: "/pilot.jpg"}},
	}
	service := NewService(repository{shows: []Show{show}, timezone: "UTC"})
	service.now = func() time.Time { return now }
	entries, err := service.Continue(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].NextEpisodeName != "Pilot" || entries[0].NextEpisodeStillPath != "/pilot.jpg" {
		t.Fatalf("episode details = %+v", entries)
	}
}

func TestRemainingEpisodes_GivenLaterRegularEpisodes_WhenCountingAfterCurrent_ThenItExcludesWatchedEpisodesAndSpecials(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	episodes := []domain.Episode{
		{ID: "special", SeasonNumber: 0, EpisodeNumber: 1, AirDate: &now},
		{ID: "current", SeasonNumber: 1, EpisodeNumber: 1, AirDate: &now},
		{ID: "later", SeasonNumber: 1, EpisodeNumber: 2, AirDate: &now},
		{ID: "watched-later", SeasonNumber: 1, EpisodeNumber: 3, AirDate: &now},
		{ID: "next-season", SeasonNumber: 2, EpisodeNumber: 1, AirDate: &now},
	}
	plays := []domain.EpisodePlay{{ID: "p1", EpisodeID: "watched-later"}}
	if got := remainingEpisodesAfter(episodes, plays, episodes[1]); got != 2 {
		t.Fatalf("remaining episodes=%d, want two later unwatched regular episodes", got)
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

func TestContinue_GivenRecentWatchAndNewRelease_WhenLoadingNow_ThenMostRecentActivityIsFirst(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	recentRelease := now.Add(-30 * time.Minute)
	recentWatch := now.Add(-2 * time.Hour)
	oldWatchedAirDate := now.Add(-48 * time.Hour)
	oldNextAirDate := now.Add(-24 * time.Hour)
	shows := []Show{
		{ID: "tv:older-watch", Title: "Older watch", UpdatedAt: now.Add(-72 * time.Hour), Episodes: []domain.Episode{{ID: "old-watched", SeasonNumber: 1, EpisodeNumber: 1, AirDate: &oldWatchedAirDate}, {ID: "old-next", SeasonNumber: 1, EpisodeNumber: 2, AirDate: &oldNextAirDate}}, Plays: []domain.EpisodePlay{{ID: "old-play", EpisodeID: "old-watched", WatchedAt: recentWatch}}},
		{ID: "tv:new-release", Title: "New release", UpdatedAt: now.Add(-7 * 24 * time.Hour), Episodes: []domain.Episode{{ID: "new-next", SeasonNumber: 1, EpisodeNumber: 1, AirDate: &recentRelease}}},
	}
	service := NewService(repository{shows: shows, timezone: "UTC"})
	service.now = func() time.Time { return now }
	entries, err := service.Continue(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].ShowID != "tv:new-release" || entries[1].ShowID != "tv:older-watch" {
		t.Fatalf("order=%v, want new-release before older-watch", []string{entries[0].ShowID, entries[1].ShowID})
	}
}

func TestRefreshCatalog_GivenManyStaleShows_WhenSchedulerRefreshesCatalog_ThenProviderWorkIsCapped(t *testing.T) {
	repository := &metadataRepository{showIDs: []int64{10, 11, 12, 13, 14}}
	provider := &showMetadataProvider{}
	service := NewService(repository, provider)

	if err := service.RefreshCatalog(context.Background(), 24*time.Hour, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if len(provider.refreshCalls) != maxShowRefreshesPerRequest || len(provider.calls) != 0 || len(repository.imported) != maxShowRefreshesPerRequest {
		t.Fatalf("bounded provider calls=%v full provider calls=%v imports=%v, want at most %d bounded refreshes for one request", provider.refreshCalls, provider.calls, repository.imported, maxShowRefreshesPerRequest)
	}
	if repository.refreshLimit != maxShowRefreshesPerRequest {
		t.Fatalf("refresh query limit=%d, want %d", repository.refreshLimit, maxShowRefreshesPerRequest)
	}
	if provider.refreshCalls[0] != 10 || provider.refreshCalls[1] != 11 {
		t.Fatalf("refresh order=%v, want most recently tracked shows first", provider.refreshCalls)
	}
}
