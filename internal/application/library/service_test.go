package library_test

import (
	"context"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
	"testing"
)

type repo struct {
	item    library.Item
	media   library.Media
	entries []library.Entry
}

func (r *repo) UpsertItem(_ context.Context, item library.Item) error          { r.item = item; return nil }
func (r *repo) UpsertMedia(_ context.Context, media library.Media) error       { r.media = media; return nil }
func (r *repo) ListItems(_ context.Context, _ string) ([]library.Entry, error) { return r.entries, nil }
func TestSave_GivenFiveStarRating_WhenSaving_ThenItPersists(t *testing.T) {
	r := &repo{}
	rating := 5
	item, err := library.NewService(r).Save(context.Background(), "u", "m", domain.WatchingStatus, &rating)
	if err != nil {
		t.Fatal(err)
	}
	if item.Rating == nil || *item.Rating != 5 {
		t.Fatalf("rating=%v", item.Rating)
	}
}

func TestSaveMedia_GivenTMDBMovie_WhenSaving_ThenItUsesStableMediaID(t *testing.T) {
	r := &repo{}
	_, err := library.NewService(r).SaveMedia(context.Background(), "u", library.Media{Type: domain.MovieMediaType, TMDBID: 42, Title: "Answer"}, domain.WatchlistStatus, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.media.ID != "movie:42" || r.item.MediaID != "movie:42" {
		t.Fatalf("media IDs = %q and %q", r.media.ID, r.item.MediaID)
	}
}
func TestSave_GivenSixStarRating_WhenSaving_ThenItRejectsIt(t *testing.T) {
	rating := 6
	_, err := library.NewService(&repo{}).Save(context.Background(), "u", "m", domain.WatchingStatus, &rating)
	if err == nil {
		t.Fatal("expected rating error")
	}
}
