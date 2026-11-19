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
	tmdbIDs, err := repository.ShowsNeedingCatalogRefresh(ctx, activeTTL, finishedTTL, service.showBatchSize)
	if err != nil {
		return err
	}

	for _, tmdbID := range tmdbIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		refresh := func() error {
			// Recheck after joining an overlapping detail-page refresh.
			ids, err := repository.ShowsNeedingCatalogRefresh(ctx, activeTTL, finishedTTL, service.showBatchSize)
			if err != nil {
				return err
			}
			found := false
			for _, id := range ids {
				if id == tmdbID {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
			return service.refreshCatalogShow(ctx, repository, tmdbID)
		}
		var err error
		if once, ok := service.repository.(interface {
			RefreshMetadataOnce(context.Context, string, func() error) error
		}); ok {
			err = once.RefreshMetadataOnce(ctx, fmt.Sprintf("tv:%d", tmdbID), refresh)
		} else {
			err = refresh()
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("scheduled catalog refresh skipped tv:%d: %v", tmdbID, err)
		}
	}

	if provider, ok := service.metadataProvider.(domain.MovieMetadataProvider); ok {
		if repository, ok := service.repository.(interface {
			MoviesNeedingReleaseRefresh(context.Context, time.Duration, int) ([]int64, error)
			RefreshMovieMetadata(context.Context, domain.MovieMetadata) error
		}); ok {
			movieIDs, err := repository.MoviesNeedingReleaseRefresh(ctx, activeTTL, service.movieBatchSize)
			if err != nil {
				return err
			}

			for _, tmdbID := range movieIDs {
				if err := ctx.Err(); err != nil {
					return err
				}
				refresh := func() error {
					ids, err := repository.MoviesNeedingReleaseRefresh(ctx, activeTTL, service.movieBatchSize)
					if err != nil {
						return err
					}
					found := false
					for _, id := range ids {
						if id == tmdbID {
							found = true
							break
						}
					}
					if !found {
						return nil
					}
					started := time.Now().UTC()
					var movie domain.MovieMetadata
					if fresh, ok := provider.(interface {
						FreshMovie(context.Context, int64) (domain.MovieMetadata, error)
					}); ok {
						movie, err = fresh.FreshMovie(ctx, tmdbID)
					} else {
						movie, err = provider.Movie(ctx, tmdbID)
					}
					if err != nil {
						return err
					}
					if movie.TMDBID != tmdbID || movie.Title == "" {
						return fmt.Errorf("invalid movie metadata")
					}
					movie.FetchStartedAt = started
					return repository.RefreshMovieMetadata(ctx, movie)
				}
				var err error
				if once, ok := service.repository.(interface {
					RefreshMetadataOnce(context.Context, string, func() error) error
				}); ok {
					err = once.RefreshMetadataOnce(ctx, fmt.Sprintf("movie:%d", tmdbID), refresh)
				} else {
					err = refresh()
				}
				if err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					log.Printf("scheduled catalog refresh skipped movie:%d: %v", tmdbID, err)
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

func (service *Service) refreshCatalogShow(ctx context.Context, repository catalogRefreshRepository, tmdbID int64) error {
	started := time.Now().UTC()
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
		if fresh, ok := service.metadataProvider.(interface {
			FreshShow(context.Context, int64) (domain.TVShowMetadata, error)
		}); ok {
			metadata, err = fresh.FreshShow(ctx, tmdbID)
		} else {
			metadata, err = service.metadataProvider.Show(ctx, tmdbID)
		}
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
			var season domain.TVSeasonMetadata
			var err error
			if fresh, ok := provider.(interface {
				FreshSeason(context.Context, int64, int) (domain.TVSeasonMetadata, error)
			}); ok {
				season, err = fresh.FreshSeason(ctx, tmdbID, number)
			} else {
				season, err = provider.Season(ctx, tmdbID, number)
			}
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
	if metadata.TMDBID != tmdbID || metadata.Name == "" {
		return fmt.Errorf("invalid show metadata")
	}
	metadata.FetchStartedAt = started
	for i := range metadata.Seasons {
		if metadata.Seasons[i].FetchStartedAt.IsZero() {
			metadata.Seasons[i].FetchStartedAt = started
		}
	}
	if err := repository.ImportShowMetadata(ctx, fmt.Sprintf("tv:%d", tmdbID), metadata); err != nil {
		return err
	}
	return nil
}
