package feed

import (
	"context"
	"errors"
	"testing"
)

type feedRepository struct {
	cursor string
	limit  int
	err    error
}

func (repository *feedRepository) List(_ context.Context, cursor string, limit int) (Page, error) {
	repository.cursor, repository.limit = cursor, limit
	return Page{Items: []Item{{ID: "activity-1"}}}, repository.err
}

func TestList_GivenDefaultAndExplicitLimits_WhenListingActivity_ThenItBoundsAndForwardsTheRequest(t *testing.T) {
	repository := &feedRepository{}
	service := NewService(repository)
	page, err := service.List(context.Background(), "next", 0)
	if err != nil || repository.cursor != "next" || repository.limit != 30 || len(page.Items) != 1 {
		t.Fatalf("page=%+v cursor=%q limit=%d err=%v", page, repository.cursor, repository.limit, err)
	}
	if _, err := service.List(context.Background(), "", 101); err == nil {
		t.Fatal("expected oversized page to be rejected")
	}
}

func TestList_GivenARepositoryFailure_WhenListingActivity_ThenItReturnsTheFailure(t *testing.T) {
	want := errors.New("database unavailable")
	service := NewService(&feedRepository{err: want})
	if _, err := service.List(context.Background(), "", 10); !errors.Is(err, want) {
		t.Fatalf("error=%v, want %v", err, want)
	}
}
