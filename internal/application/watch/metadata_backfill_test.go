package watch

import (
	"context"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

type wakingRepository struct {
	repository
	scans chan struct{}
}

func (r *wakingRepository) MissingMetadataAfter(context.Context, MissingMedia, int) ([]MissingMedia, error) {
	r.scans <- struct{}{}
	return nil, nil
}
func (*wakingRepository) ImportShowMetadata(context.Context, string, domain.TVShowMetadata) error {
	return nil
}
func (*wakingRepository) SaveMovieMetadata(context.Context, domain.MovieMetadata) error { return nil }

func TestMetadataBackfillRunsAtStartupAndOnWake(t *testing.T) {
	repo := &wakingRepository{scans: make(chan struct{}, 2)}
	service := NewService(repo, &showMetadataProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { service.RunMetadataBackfill(ctx); close(done) }()
	awaitScan := func() {
		select {
		case <-repo.scans:
		case <-time.After(5 * time.Second):
			t.Fatal("backfill did not scan")
		}
	}
	awaitScan()
	service.WakeMetadataBackfill()
	awaitScan()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("backfill did not stop")
	}
}
