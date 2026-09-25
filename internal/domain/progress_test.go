package domain_test

import (
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

func TestCalculateShowProgress_GivenSkippedEarlySeasons_WhenLaterEpisodeWasWatched_ThenCursorFollowsLaterEpisode(t *testing.T) {
	now := date("2026-09-24")
	episodes := []domain.Episode{
		episode("s1e1", 1, 1, "2024-01-01"),
		episode("s15e20", 15, 20, "2026-09-01"),
		episode("s16e1", 16, 1, "2026-10-01"),
	}
	plays := []domain.EpisodePlay{{EpisodeID: "s15e20", WatchedAt: now}}

	progress := domain.CalculateShowProgress(episodes, plays, now)

	if progress.Cursor == nil || progress.Cursor.ID != "s15e20" {
		t.Fatalf("cursor = %#v, want s15e20", progress.Cursor)
	}
	if progress.NextEpisode != nil {
		t.Fatalf("next episode = %#v, want nil while caught up", progress.NextEpisode)
	}
	if !progress.IsCaughtUp {
		t.Fatal("show should be caught up")
	}
}

func TestCalculateShowProgress_GivenNewEpisodeAfterCursor_WhenItHasAired_ThenItBecomesNextEpisode(t *testing.T) {
	now := date("2026-10-02")
	episodes := []domain.Episode{
		episode("s15e20", 15, 20, "2026-09-01"),
		episode("s16e1", 16, 1, "2026-10-01"),
	}

	progress := domain.CalculateShowProgress(episodes, []domain.EpisodePlay{{EpisodeID: "s15e20"}}, now)

	if progress.NextEpisode == nil || progress.NextEpisode.ID != "s16e1" {
		t.Fatalf("next episode = %#v, want s16e1", progress.NextEpisode)
	}
	if progress.IsCaughtUp {
		t.Fatal("show should no longer be caught up")
	}
}

func TestCalculateShowProgress_GivenUnwatchedSpecial_WhenRegularEpisodesAreWatched_ThenSpecialDoesNotBlockProgress(t *testing.T) {
	now := date("2026-09-24")
	episodes := []domain.Episode{
		episode("special", 0, 1, "2026-01-01"),
		episode("s1e1", 1, 1, "2026-01-02"),
	}

	progress := domain.CalculateShowProgress(episodes, []domain.EpisodePlay{{EpisodeID: "s1e1"}}, now)

	if !progress.IsCaughtUp {
		t.Fatal("an unwatched special must not block progress")
	}
}

func TestMissingPriorEpisodes_GivenLaterSelectedEpisode_WhenEarlierEpisodesAreUnwatched_ThenOnlyReleasedRegularGapsAreReturned(t *testing.T) {
	now := date("2026-09-24")
	episodes := []domain.Episode{
		episode("special", 0, 1, "2026-01-01"),
		episode("s1e1", 1, 1, "2026-01-02"),
		episode("s1e2", 1, 2, "2026-01-03"),
		episode("s1e3", 1, 3, "2026-10-01"),
		episode("s2e1", 2, 1, "2026-09-01"),
	}

	missing := domain.MissingPriorEpisodes(episodes, []domain.EpisodePlay{{EpisodeID: "s1e2"}}, episodes[4], now)

	if len(missing) != 1 || missing[0].ID != "s1e1" {
		t.Fatalf("missing = %#v, want only s1e1", missing)
	}
}

func TestMissingEpisodesThrough_IncludesUnwatchedTargetAndSkipsFutureAndSpecials(t *testing.T) {
	now := date("2026-09-24")
	episodes := []domain.Episode{
		episode("special", 0, 1, "2026-01-01"),
		episode("s1e1", 1, 1, "2026-01-02"),
		episode("s1e2", 1, 2, "2026-01-03"),
		episode("s1e3", 1, 3, "2026-10-01"),
	}
	missing := domain.MissingEpisodesThrough(episodes, []domain.EpisodePlay{{EpisodeID: "s1e1"}}, episodes[2], now)
	if len(missing) != 1 || missing[0].ID != "s1e2" {
		t.Fatalf("missing=%#v, want only target s1e2", missing)
	}
}

func episode(id string, season, number int, airDate string) domain.Episode {
	date := date(airDate)
	return domain.Episode{ID: id, SeasonNumber: season, EpisodeNumber: number, AirDate: &date}
}

func date(value string) time.Time {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return parsed
}
