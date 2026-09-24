package export

import (
	"context"
	"errors"
	"testing"
	"time"
)

type exportRepository struct {
	userID string
	err    error
	data   Data
}

func (repository *exportRepository) Export(_ context.Context, userID string) (Data, error) {
	repository.userID = userID
	return repository.data, repository.err
}

func TestData_GivenAUserAndRepositoryData_WhenExporting_ThenItReturnsTheDataWithAControlledTimestamp(t *testing.T) {
	repository := &exportRepository{data: Data{Library: []LibraryItem{{MediaID: "movie:1", Title: "Example"}}}}
	service := NewService(repository)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	data, err := service.Data(context.Background(), "user-1")
	if err != nil || repository.userID != "user-1" || data.ExportedAt != now || len(data.Library) != 1 {
		t.Fatalf("data=%+v user=%q err=%v", data, repository.userID, err)
	}
}

func TestData_GivenMissingUserOrRepositoryFailure_WhenExporting_ThenItReturnsAnError(t *testing.T) {
	service := NewService(&exportRepository{})
	if _, err := service.Data(context.Background(), ""); err == nil {
		t.Fatal("expected missing user to be rejected")
	}
	want := errors.New("database unavailable")
	service = NewService(&exportRepository{err: want})
	if _, err := service.Data(context.Background(), "user-1"); !errors.Is(err, want) {
		t.Fatalf("error=%v, want %v", err, want)
	}
}
