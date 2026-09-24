package library_test

import (
	"context"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
	"testing"
)

type repo struct{ item library.Item }

func (r *repo) UpsertItem(_ context.Context, item library.Item) error { r.item = item; return nil }
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
func TestSave_GivenSixStarRating_WhenSaving_ThenItRejectsIt(t *testing.T) {
	rating := 6
	_, err := library.NewService(&repo{}).Save(context.Background(), "u", "m", domain.WatchingStatus, &rating)
	if err == nil {
		t.Fatal("expected rating error")
	}
}
