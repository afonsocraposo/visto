package watch

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

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
		if boundedProvider, ok := service.metadataProvider.(domain.ScheduledTVShowMetadataProvider); ok {
			metadata, err = boundedProvider.RefreshShow(ctx, tmdbID)
		} else {
			metadata, err = service.metadataProvider.Show(ctx, tmdbID)
		}
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
