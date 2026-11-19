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

func (service *Service) refreshSeason(ctx context.Context, seasonID string) error {
	repo, ok := service.repository.(interface {
		SeasonMetadataNeedsRefresh(context.Context, string, time.Duration) (int64, int, bool, error)
		RefreshSeasonMetadata(context.Context, string, domain.TVSeasonMetadata) error
		RefreshMetadataOnce(context.Context, string, func() error) error
	})
	provider, configured := service.metadataProvider.(domain.TVShowSummaryProvider)
	if !ok || !configured {
		return nil
	}
	return repo.RefreshMetadataOnce(ctx, seasonID, func() error {
		id, number, stale, err := repo.SeasonMetadataNeedsRefresh(ctx, seasonID, service.metadataTTL)
		if err != nil || !stale {
			return err
		}
		started := time.Now().UTC()
		var season domain.TVSeasonMetadata
		if fresh, ok := provider.(interface {
			FreshSeason(context.Context, int64, int) (domain.TVSeasonMetadata, error)
		}); ok {
			season, err = fresh.FreshSeason(ctx, id, number)
		} else {
			season, err = provider.Season(ctx, id, number)
		}
		if err != nil || season.Number != number || season.Episodes == nil {
			return nil
		}
		for _, episode := range season.Episodes {
			if episode.SeasonNumber != number || episode.EpisodeNumber <= 0 {
				return nil
			}
		}
		season.FetchStartedAt = started
		return repo.RefreshSeasonMetadata(ctx, fmt.Sprintf("tv:%d", id), season)
	})
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

// EpisodeDetails keeps saved supplementary metadata durable; browsing never creates episodes.
func (service *Service) EpisodeDetails(ctx context.Context, id int64, season, number int, provider domain.TVEpisodeMetadataProvider) (domain.TVEpisodeMetadata, error) {
	repo, ok := service.repository.(interface {
		EpisodeMetadata(context.Context, int64, int, int, time.Duration) (domain.TVEpisodeMetadata, bool, bool, error)
		RefreshEpisodeMetadata(context.Context, int64, domain.TVEpisodeMetadata) error
		RefreshMetadataOnce(context.Context, string, func() error) error
	})
	if !ok {
		return provider.Episode(ctx, id, season, number)
	}
	episode, exists, _, err := repo.EpisodeMetadata(ctx, id, season, number, service.metadataTTL)
	if err != nil {
		return episode, err
	}
	if !exists {
		return provider.Episode(ctx, id, season, number)
	}
	key := fmt.Sprintf("tv:%d:season:%d:episode:%d:details", id, season, number)
	err = repo.RefreshMetadataOnce(ctx, key, func() error {
		stored, exists, stale, err := repo.EpisodeMetadata(ctx, id, season, number, service.metadataTTL)
		if err != nil || !exists || !stale {
			return err
		}
		started := time.Now().UTC()
		var updated domain.TVEpisodeMetadata
		if fresh, ok := provider.(interface {
			FreshEpisode(context.Context, int64, int, int) (domain.TVEpisodeMetadata, error)
		}); ok {
			updated, err = fresh.FreshEpisode(ctx, id, season, number)
		} else {
			updated, err = provider.Episode(ctx, id, season, number)
		}
		if err != nil || updated.TMDBID <= 0 || updated.Name == "" || updated.SeasonNumber != season || updated.EpisodeNumber != number || (stored.TMDBID > 0 && stored.TMDBID != updated.TMDBID) {
			return nil
		}
		updated.FetchStartedAt = started
		return repo.RefreshEpisodeMetadata(ctx, id, updated)
	})
	if err != nil {
		return episode, err
	}
	episode, _, _, err = repo.EpisodeMetadata(ctx, id, season, number, service.metadataTTL)
	return episode, err
}
