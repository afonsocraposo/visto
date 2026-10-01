package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestMetadataProvidersWithoutTMDBAreNil(t *testing.T) {
	providers := newMetadataProviders(nil)
	if providers.metadata != nil || providers.shows != nil || providers.importResolver != nil || providers.plex != nil {
		t.Fatalf("providers without TMDB = %#v, want nil interfaces", providers)
	}
}

// Regression: a nil *tmdb.Client wrapped in an interface made the background
// metadata jobs call TMDB methods on nil and crash the server at startup.
func TestBackgroundMetadataJobsRunWithoutTMDB(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer store.Close()
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at)
		VALUES('tv:1','tv',1,'Placeholder','','2026-01-01T00:00:00Z'), ('movie:2','movie',2,'Placeholder','','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert placeholders: %v", err)
	}
	service := watch.NewService(store, newMetadataProviders(nil).shows...)
	if err := service.BackfillMissingMetadata(ctx); err != nil {
		t.Fatalf("backfill without TMDB: %v", err)
	}
	if err := service.RefreshCatalog(ctx, time.Hour, time.Hour); err != nil {
		t.Fatalf("catalog refresh without TMDB: %v", err)
	}
}
