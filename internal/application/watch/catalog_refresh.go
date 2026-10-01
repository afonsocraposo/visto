package watch

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

// RefreshCatalog refreshes bounded tracked media catalogs from the backend scheduler.
func (service *Service) RefreshCatalog(ctx context.Context, activeTTL, finishedTTL time.Duration) error {
	if service.metadataProvider == nil {
		return nil
	}
	repository, ok := service.repository.(catalogRefreshRepository)
	if !ok {
		return nil
	}
	tmdbIDs, err := repository.ShowsNeedingCatalogRefresh(ctx, activeTTL, finishedTTL, maxShowRefreshesPerRequest)
	if err != nil {
		return err
	}
	for _, tmdbID := range tmdbIDs {
		var metadata domain.TVShowMetadata
		var err error
		missing := false
		if detector, ok := service.repository.(interface {
			ShowMetadataMissing(context.Context, int64) (bool, error)
		}); ok {
			missing, err = detector.ShowMetadataMissing(ctx, tmdbID)
			if err != nil {
				return err
			}
		}
		if boundedProvider, ok := service.metadataProvider.(domain.ScheduledTVShowMetadataProvider); ok && !missing {
			metadata, err = boundedProvider.RefreshShow(ctx, tmdbID)
		} else {
			metadata, err = service.metadataProvider.Show(ctx, tmdbID)
		}
		if err != nil {
			return err
		}
		if provider, ok := service.metadataProvider.(domain.TVShowSummaryProvider); ok {
			showID := fmt.Sprintf("tv:%d", tmdbID)
			var numbers []int
			if pending, ok := service.repository.(interface {
				PendingSeasonNumbers(context.Context, string) ([]int, error)
			}); ok {
				numbers, err = pending.PendingSeasonNumbers(ctx, showID)
				if err != nil {
					return err
				}
			}
			if catalog, ok := service.repository.(interface {
				SeasonCatalogStates(context.Context, string) ([]SeasonCatalogState, error)
			}); ok {
				states, err := catalog.SeasonCatalogStates(ctx, showID)
				if err != nil {
					return err
				}
				byNumber := make(map[int]SeasonCatalogState, len(states))
				for _, state := range states {
					byNumber[state.SeasonNumber] = state
				}
				for _, season := range metadata.Seasons {
					state := byNumber[season.Number]
					if season.EpisodeCount > 0 && season.Episodes == nil &&
						(state.ActiveEpisodeCount != season.EpisodeCount || state.SyntheticEpisodeCount > 0) {
						numbers = append(numbers, season.Number)
					}
				}
			}
			fetched := make(map[int]bool, len(numbers))
			for _, number := range numbers {
				if fetched[number] {
					continue
				}
				fetched[number] = true
				replaced := false
				for _, season := range metadata.Seasons {
					if season.Number == number && season.Episodes != nil {
						replaced = true
						break
					}
				}
				if replaced {
					continue
				}
				season, err := provider.Season(ctx, tmdbID, number)
				if err != nil {
					return err
				}
				for i := range metadata.Seasons {
					if metadata.Seasons[i].Number == number {
						metadata.Seasons[i] = season
						replaced = true
						break
					}
				}
				if !replaced {
					metadata.Seasons = append(metadata.Seasons, season)
				}
			}
		}
		if err := repository.ImportShowMetadata(ctx, fmt.Sprintf("tv:%d", tmdbID), metadata); err != nil {
			return err
		}
	}
	if provider, ok := service.metadataProvider.(domain.MovieMetadataProvider); ok {
		if repository, ok := service.repository.(interface {
			MoviesNeedingReleaseRefresh(context.Context, time.Duration, int) ([]int64, error)
			RefreshMovieMetadata(context.Context, domain.MovieMetadata) error
		}); ok {
			movieIDs, err := repository.MoviesNeedingReleaseRefresh(ctx, activeTTL, maxShowRefreshesPerRequest)
			if err != nil {
				return err
			}
			for _, tmdbID := range movieIDs {
				movie, err := provider.Movie(ctx, tmdbID)
				if err != nil {
					return err
				}
				if movie.TMDBID != tmdbID || movie.Title == "" {
					return fmt.Errorf("invalid movie metadata")
				}
				if err := repository.RefreshMovieMetadata(ctx, movie); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// RunCatalogRefresher keeps local metadata current without coupling TMDB
// traffic to page visits. The caller owns ctx and controls shutdown.
func (service *Service) RunCatalogRefresher(ctx context.Context, interval, activeTTL, finishedTTL time.Duration) {
	refresh := func() {
		if err := service.RefreshCatalog(ctx, activeTTL, finishedTTL); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("scheduled catalog refresh failed: %v", err)
		}
	}
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
