package watch

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/afonsocosta/visto/internal/domain"
)

type MissingMedia struct {
	Type   domain.MediaType
	TMDBID int64
}

type backfillRepository interface {
	MissingMetadataAfter(context.Context, MissingMedia, int) ([]MissingMedia, error)
	ImportShowMetadata(context.Context, string, domain.TVShowMetadata) error
	SaveMovieMetadata(context.Context, domain.MovieMetadata) error
}

func (service *Service) WakeMetadataBackfill() {
	select {
	case service.backfillWake <- struct{}{}:
	default:
	}
}

// BackfillMissingMetadata attempts each placeholder once per wake, in small batches.
func (service *Service) BackfillMissingMetadata(ctx context.Context) error {
	repository, ok := service.repository.(backfillRepository)
	if !ok || service.metadataProvider == nil {
		return nil
	}
	cursor := MissingMedia{}
	for {
		batch, err := repository.MissingMetadataAfter(ctx, cursor, maxShowRefreshesPerRequest)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		for _, item := range batch {
			cursor = item
			if err := ctx.Err(); err != nil {
				return err
			}
			if item.Type == domain.TVMediaType {
				show, err := service.metadataProvider.Show(ctx, item.TMDBID)
				if err == nil && (show.TMDBID != item.TMDBID || show.Name == "") {
					err = fmt.Errorf("invalid show metadata")
				}
				if err == nil {
					err = repository.ImportShowMetadata(ctx, fmt.Sprintf("tv:%d", item.TMDBID), show)
				}
				if err != nil {
					log.Printf("metadata backfill skipped tv:%d: %v", item.TMDBID, err)
				}
				continue
			}
			provider, ok := service.metadataProvider.(domain.MovieMetadataProvider)
			if !ok {
				continue
			}
			movie, err := provider.Movie(ctx, item.TMDBID)
			if err == nil && (movie.TMDBID != item.TMDBID || movie.Title == "") {
				err = fmt.Errorf("invalid movie metadata")
			}
			if err == nil {
				err = repository.SaveMovieMetadata(ctx, movie)
			}
			if err != nil {
				log.Printf("metadata backfill skipped movie:%d: %v", item.TMDBID, err)
			}
		}
	}
}

func (service *Service) RunMetadataBackfill(ctx context.Context) {
	run := func() {
		if err := service.BackfillMissingMetadata(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("metadata backfill failed: %v", err)
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-service.backfillWake:
			run()
		}
	}
}
