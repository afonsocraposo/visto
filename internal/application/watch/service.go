package watch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

const maxShowRefreshesPerRequest = 2

type Show struct {
	ID             string                    `json:"id"`
	Title          string                    `json:"title"`
	PosterPath     string                    `json:"poster_path"`
	Status         domain.LibraryStatus      `json:"status"`
	Episodes       []domain.Episode          `json:"-"`
	EpisodeDetails map[string]EpisodeDisplay `json:"-"`
	Plays          []domain.EpisodePlay      `json:"-"`
	UpdatedAt      time.Time                 `json:"-"`
}

type EpisodeDisplay struct {
	Name      string
	StillPath string
}

type ContinueEntry struct {
	ShowID               string           `json:"show_id"`
	Title                string           `json:"title"`
	PosterPath           string           `json:"poster_path"`
	Kind                 string           `json:"kind"`
	Cursor               *domain.Episode  `json:"cursor,omitempty"`
	NextEpisode          *domain.Episode  `json:"next_episode,omitempty"`
	NextEpisodeName      string           `json:"next_episode_name,omitempty"`
	NextEpisodeStillPath string           `json:"next_episode_still_path,omitempty"`
	RemainingEpisodes    int              `json:"remaining_episodes"`
	MissingPriorEpisodes []domain.Episode `json:"missing_prior_episodes,omitempty"`
}

type CalendarEntry struct {
	ShowID  string         `json:"show_id"`
	Title   string         `json:"title"`
	Episode domain.Episode `json:"episode"`
}

type ShowEpisode struct {
	Episode   domain.Episode `json:"episode"`
	Name      string         `json:"name"`
	Overview  string         `json:"overview,omitempty"`
	Runtime   int            `json:"runtime,omitempty"`
	StillPath string         `json:"still_path,omitempty"`
	Watched   bool           `json:"watched"`
}

type Season struct {
	ID           string `json:"id"`
	ShowID       string `json:"show_id"`
	Number       int    `json:"season_number"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episode_count"`
	AirDate      string `json:"air_date,omitempty"`
}

type Progress struct {
	ShowID                string          `json:"show_id"`
	Cursor                *domain.Episode `json:"cursor"`
	NextEpisode           *domain.Episode `json:"next_episode"`
	LatestReleasedEpisode *domain.Episode `json:"latest_released_episode"`
	IsCaughtUp            bool            `json:"is_caught_up"`
	IsFullyWatched        bool            `json:"is_fully_watched"`
	WatchedEpisodes       int             `json:"watched_episodes"`
	TotalEpisodes         int             `json:"total_episodes"`
	ReleasedEpisodes      int             `json:"released_episodes"`
	MissingPriorEpisodes  int             `json:"missing_prior_episodes"`
}

type Repository interface {
	WatchingShows(context.Context, string) ([]Show, error)
	Timezone(context.Context, string) (string, error)
}

type episodeRepository interface {
	ListShowEpisodes(context.Context, string, string) ([]ShowEpisode, error)
	ListShowSeasons(context.Context, string, string) ([]Season, error)
	ListSeasonEpisodes(context.Context, string, string) ([]ShowEpisode, error)
}

var ErrShowNotFound = errors.New("show not found")

type catalogRefreshRepository interface {
	ShowsNeedingCatalogRefresh(context.Context, time.Duration, time.Duration, int) ([]int64, error)
	ImportShowMetadata(context.Context, string, domain.TVShowMetadata) error
}

type Service struct {
	repository       Repository
	now              func() time.Time
	metadataProvider domain.TVShowMetadataProvider
}

func NewService(repository Repository, providers ...domain.TVShowMetadataProvider) *Service {
	service := &Service{repository: repository, now: time.Now}
	if len(providers) > 0 {
		service.metadataProvider = providers[0]
	}
	return service
}

func (service *Service) Continue(ctx context.Context, userID string) ([]ContinueEntry, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	shows, err := service.repository.WatchingShows(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := service.now().UTC()
	zoneName, err := service.repository.Timezone(ctx, userID)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(zoneName)
	if err != nil {
		return nil, fmt.Errorf("invalid user timezone")
	}
	now = now.In(location)
	result := []ContinueEntry{}
	for _, show := range shows {
		progress := domain.CalculateShowProgress(show.Episodes, show.Plays, now)
		if progress.NextEpisode == nil {
			continue
		}
		kind := "continue"
		if progress.Cursor == nil {
			kind = "start"
		}
		entry := ContinueEntry{ShowID: show.ID, Title: show.Title, PosterPath: show.PosterPath, Kind: kind, Cursor: progress.Cursor, NextEpisode: progress.NextEpisode}
		if details, ok := show.EpisodeDetails[progress.NextEpisode.ID]; ok {
			entry.NextEpisodeName = details.Name
			entry.NextEpisodeStillPath = details.StillPath
		}
		entry.RemainingEpisodes = remainingEpisodesAfter(show.Episodes, show.Plays, *progress.NextEpisode)
		entry.MissingPriorEpisodes = domain.MissingPriorEpisodes(show.Episodes, show.Plays, *progress.NextEpisode, now)
		result = append(result, entry)
	}
	sort.SliceStable(result, func(left, right int) bool {
		return continueActivityTime(shows, result[left], now).After(continueActivityTime(shows, result[right], now))
	})
	return result, nil
}

func remainingEpisodesAfter(episodes []domain.Episode, plays []domain.EpisodePlay, current domain.Episode) int {
	played := make(map[string]struct{}, len(plays))
	for _, play := range plays {
		played[play.EpisodeID] = struct{}{}
	}
	remaining := 0
	for _, episode := range episodes {
		if !episode.IsRegular() || episode.SeasonNumber < current.SeasonNumber || episode.SeasonNumber == current.SeasonNumber && episode.EpisodeNumber <= current.EpisodeNumber {
			continue
		}
		if _, watched := played[episode.ID]; !watched {
			remaining++
		}
	}
	return remaining
}

func continueActivityTime(shows []Show, entry ContinueEntry, now time.Time) time.Time {
	for _, show := range shows {
		if show.ID != entry.ShowID {
			continue
		}
		latest := show.UpdatedAt
		for _, play := range show.Plays {
			if play.WatchedAt.After(latest) {
				latest = play.WatchedAt
			}
		}
		if entry.NextEpisode != nil && entry.NextEpisode.AirDate != nil && entry.NextEpisode.IsReleasedAt(now) && entry.NextEpisode.AirDate.After(latest) {
			latest = *entry.NextEpisode.AirDate
		}
		return latest
	}
	return time.Time{}
}
