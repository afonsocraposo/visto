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
	ShowsNeedingCatalogRefresh(context.Context, time.Duration, time.Duration) ([]int64, error)
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
		entry.MissingPriorEpisodes = domain.MissingPriorEpisodes(show.Episodes, show.Plays, *progress.NextEpisode, now)
		result = append(result, entry)
	}
	sort.SliceStable(result, func(left, right int) bool {
		return continueActivityTime(shows, result[left], now).After(continueActivityTime(shows, result[right], now))
	})
	return result, nil
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

func (service *Service) Calendar(ctx context.Context, userID string, from, to time.Time) ([]CalendarEntry, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	fromProvided, toProvided := !from.IsZero(), !to.IsZero()
	zoneName, err := service.repository.Timezone(ctx, userID)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(zoneName)
	if err != nil {
		return nil, fmt.Errorf("invalid user timezone")
	}
	fromDate := service.now().In(location).Format(time.DateOnly)
	if fromProvided {
		fromDate = from.UTC().Format(time.DateOnly)
	}
	fromDateValue, err := time.Parse(time.DateOnly, fromDate)
	if err != nil {
		return nil, fmt.Errorf("invalid calendar start date")
	}
	toDate := fromDateValue.AddDate(0, 0, 30).Format(time.DateOnly)
	if toProvided {
		toDate = to.UTC().Format(time.DateOnly)
	}
	toDateValue, err := time.Parse(time.DateOnly, toDate)
	if err != nil || toDateValue.Before(fromDateValue) || toDateValue.Sub(fromDateValue) > 366*24*time.Hour {
		return nil, fmt.Errorf("calendar range must be ordered and no longer than one year")
	}
	shows, err := service.repository.WatchingShows(ctx, userID)
	if err != nil {
		return nil, err
	}
	played := map[string]map[string]bool{}
	for _, show := range shows {
		played[show.ID] = map[string]bool{}
		for _, play := range show.Plays {
			played[show.ID][play.EpisodeID] = true
		}
	}
	entries := []CalendarEntry{}
	for _, show := range shows {
		for _, episode := range show.Episodes {
			if !episode.IsRegular() || episode.AirDate == nil || played[show.ID][episode.ID] || episode.AirDate.UTC().Format("2006-01-02") < fromDate || episode.AirDate.UTC().Format("2006-01-02") > toDate {
				continue
			}
			entries = append(entries, CalendarEntry{ShowID: show.ID, Title: show.Title, Episode: episode})
		}
	}
	return entries, nil
}

// RefreshCatalog refreshes a bounded batch of TV catalogs. It is called by a
// backend scheduler, not by a frontend request.
func (service *Service) RefreshCatalog(ctx context.Context, activeTTL, finishedTTL time.Duration) error {
	if service.metadataProvider == nil {
		return nil
	}
	repository, ok := service.repository.(catalogRefreshRepository)
	if !ok {
		return nil
	}
	tmdbIDs, err := repository.ShowsNeedingCatalogRefresh(ctx, activeTTL, finishedTTL)
	if err != nil {
		return err
	}
	if len(tmdbIDs) > maxShowRefreshesPerRequest {
		tmdbIDs = tmdbIDs[:maxShowRefreshesPerRequest]
	}
	for _, tmdbID := range tmdbIDs {
		metadata, err := service.metadataProvider.Show(ctx, tmdbID)
		if err != nil {
			return err
		}
		if err := repository.ImportShowMetadata(ctx, fmt.Sprintf("tv:%d", tmdbID), metadata); err != nil {
			return err
		}
	}
	return nil
}

// RunCatalogRefresher keeps local metadata current without coupling TMDB
// traffic to page visits. The caller owns ctx and controls shutdown.
func (service *Service) RunCatalogRefresher(ctx context.Context, interval, activeTTL, finishedTTL time.Duration) {
	refresh := func() { _ = service.RefreshCatalog(ctx, activeTTL, finishedTTL) }
	refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}

func (service *Service) Episodes(ctx context.Context, userID, showID string) ([]ShowEpisode, error) {
	if userID == "" || showID == "" {
		return nil, fmt.Errorf("user and show are required")
	}
	repository, ok := service.repository.(episodeRepository)
	if !ok {
		return nil, fmt.Errorf("show episode storage is not configured")
	}
	return repository.ListShowEpisodes(ctx, userID, showID)
}

func (service *Service) Seasons(ctx context.Context, userID, showID string) ([]Season, error) {
	if userID == "" || showID == "" {
		return nil, fmt.Errorf("user and show are required")
	}
	repository, ok := service.repository.(episodeRepository)
	if !ok {
		return nil, fmt.Errorf("show season storage is not configured")
	}
	return repository.ListShowSeasons(ctx, userID, showID)
}

func (service *Service) SeasonEpisodes(ctx context.Context, userID, seasonID string) ([]ShowEpisode, error) {
	if userID == "" || seasonID == "" {
		return nil, fmt.Errorf("user and season are required")
	}
	repository, ok := service.repository.(episodeRepository)
	if !ok {
		return nil, fmt.Errorf("season episode storage is not configured")
	}
	return repository.ListSeasonEpisodes(ctx, userID, seasonID)
}

func (service *Service) ShowProgress(ctx context.Context, userID, showID string) (Progress, error) {
	entries, err := service.Episodes(ctx, userID, showID)
	if err != nil {
		return Progress{}, err
	}
	timezone, err := service.repository.Timezone(ctx, userID)
	if err != nil {
		return Progress{}, err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Progress{}, fmt.Errorf("invalid user timezone")
	}
	episodes := make([]domain.Episode, 0, len(entries))
	plans := make([]domain.EpisodePlay, 0, len(entries))
	for _, entry := range entries {
		episodes = append(episodes, entry.Episode)
		if entry.Watched {
			plans = append(plans, domain.EpisodePlay{ID: "progress:" + entry.Episode.ID, UserID: userID, EpisodeID: entry.Episode.ID})
		}
	}
	progress := domain.CalculateShowProgress(episodes, plans, service.now().In(location))
	return Progress{
		ShowID:                showID,
		Cursor:                progress.Cursor,
		NextEpisode:           progress.NextEpisode,
		LatestReleasedEpisode: progress.ReleasedEpisode,
		IsCaughtUp:            progress.IsCaughtUp,
	}, nil
}
