package watch

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
)

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
		if seasons, ok := service.repository.(interface {
			PendingSeasonNumbers(context.Context, string) ([]int, error)
		}); ok {
			if provider, ok := service.metadataProvider.(domain.TVShowSummaryProvider); ok {
				showID := fmt.Sprintf("tv:%d", tmdbID)
				numbers, err := seasons.PendingSeasonNumbers(ctx, showID)
				if err != nil {
					return err
				}
				for _, number := range numbers {
					season, err := provider.Season(ctx, tmdbID, number)
					if err != nil {
						return err
					}
					replaced := false
					for i := range metadata.Seasons {
						if metadata.Seasons[i].Number == number {
							if metadata.Seasons[i].EpisodeCount > season.EpisodeCount {
								season.EpisodeCount = metadata.Seasons[i].EpisodeCount
							}
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
		}
		if err := repository.ImportShowMetadata(ctx, fmt.Sprintf("tv:%d", tmdbID), metadata); err != nil {
			return err
		}
	}
	if movieProvider, ok := service.metadataProvider.(domain.MovieMetadataProvider); ok {
		if movies, ok := service.repository.(interface {
			MoviesNeedingMetadataRefresh(context.Context, int) ([]int64, error)
			UpsertMedia(context.Context, library.Media) error
		}); ok {
			ids, err := movies.MoviesNeedingMetadataRefresh(ctx, maxShowRefreshesPerRequest)
			if err != nil {
				return err
			}
			for _, id := range ids {
				movie, err := movieProvider.Movie(ctx, id)
				if err != nil {
					return err
				}
				if err := movies.UpsertMedia(ctx, library.Media{ID: fmt.Sprintf("movie:%d", id), Type: domain.MovieMediaType, TMDBID: id, Title: movie.Title, OriginalTitle: movie.OriginalTitle, Overview: movie.Overview, ReleaseDate: movie.ReleaseDate, PosterPath: movie.PosterPath, BackdropPath: movie.BackdropPath, OriginalLanguage: movie.OriginalLanguage, Status: movie.Status}); err != nil {
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
			log.Printf("scheduled TV metadata refresh failed: %v", err)
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
