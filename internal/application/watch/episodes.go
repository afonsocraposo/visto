package watch

import (
	"context"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

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
	now := service.now().In(location)
	progress := domain.CalculateShowProgress(episodes, plans, now)
	result := Progress{
		ShowID:                showID,
		Cursor:                progress.Cursor,
		NextEpisode:           progress.NextEpisode,
		LatestReleasedEpisode: progress.ReleasedEpisode,
		IsCaughtUp:            progress.IsCaughtUp,
	}
	watchedReleased := 0
	for _, entry := range entries {
		episode := entry.Episode
		if !episode.IsRegular() {
			continue
		}
		result.TotalEpisodes++
		if entry.Watched {
			result.WatchedEpisodes++
		}
		if !episode.IsReleasedAt(now) {
			continue
		}
		result.ReleasedEpisodes++
		if entry.Watched {
			watchedReleased++
		}
		if !entry.Watched && progress.Cursor != nil && (episode.SeasonNumber < progress.Cursor.SeasonNumber || episode.SeasonNumber == progress.Cursor.SeasonNumber && episode.EpisodeNumber <= progress.Cursor.EpisodeNumber) {
			result.MissingPriorEpisodes++
		}
	}
	result.IsFullyWatched = result.ReleasedEpisodes > 0 && result.ReleasedEpisodes == watchedReleased
	return result, nil
}
