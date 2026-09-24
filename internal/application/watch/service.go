package watch

import (
	"context"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

type Show struct {
	ID         string               `json:"id"`
	Title      string               `json:"title"`
	PosterPath string               `json:"poster_path"`
	Status     domain.LibraryStatus `json:"status"`
	Episodes   []domain.Episode     `json:"-"`
	Plays      []domain.EpisodePlay `json:"-"`
}

type ContinueEntry struct {
	ShowID               string           `json:"show_id"`
	Title                string           `json:"title"`
	PosterPath           string           `json:"poster_path"`
	Kind                 string           `json:"kind"`
	Cursor               *domain.Episode  `json:"cursor,omitempty"`
	NextEpisode          *domain.Episode  `json:"next_episode,omitempty"`
	MissingPriorEpisodes []domain.Episode `json:"missing_prior_episodes,omitempty"`
}

type CalendarEntry struct {
	ShowID  string         `json:"show_id"`
	Title   string         `json:"title"`
	Episode domain.Episode `json:"episode"`
}

type Repository interface {
	WatchingShows(context.Context, string) ([]Show, error)
	Timezone(context.Context, string) (string, error)
}

type refreshRepository interface {
	ShowsNeedingMetadataRefresh(context.Context, string, time.Duration) ([]int64, error)
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
	_ = service.refreshMetadata(ctx, userID)
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
		entry.MissingPriorEpisodes = domain.MissingPriorEpisodes(show.Episodes, show.Plays, *progress.NextEpisode, now)
		result = append(result, entry)
	}
	return result, nil
}

func (service *Service) Calendar(ctx context.Context, userID string, from, to time.Time) ([]CalendarEntry, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	_ = service.refreshMetadata(ctx, userID)
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

func (service *Service) refreshMetadata(ctx context.Context, userID string) error {
	if service.metadataProvider == nil {
		return nil
	}
	repository, ok := service.repository.(refreshRepository)
	if !ok {
		return nil
	}
	tmdbIDs, err := repository.ShowsNeedingMetadataRefresh(ctx, userID, 24*time.Hour)
	if err != nil {
		return err
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
