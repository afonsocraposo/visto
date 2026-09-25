package activity

import (
	"context"
	"errors"
	"testing"
	"time"
)

type retentionRepository struct {
	cutoff time.Time
	limits []int
	counts []int
	err    error
}

func (repository *retentionRepository) CleanupActivityEvents(_ context.Context, cutoff time.Time, limit int) (int, error) {
	repository.cutoff = cutoff
	repository.limits = append(repository.limits, limit)
	if repository.err != nil {
		return 0, repository.err
	}
	index := len(repository.limits) - 1
	if index >= len(repository.counts) {
		return 0, nil
	}
	return repository.counts[index], nil
}

func TestCleanup_GivenExpiredActivityAndBacklog_WhenRun_ThenItUsesRecordedTimeAndBoundsBatches(t *testing.T) {
	repository := &retentionRepository{counts: []int{CleanupBatchSize, 12}}
	service := NewRetentionService(repository)
	service.now = func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.FixedZone("local", 2*60*60)) }

	removed, err := service.Cleanup(context.Background(), 365*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	wantCutoff := time.Date(2025, 9, 25, 10, 0, 0, 0, time.UTC)
	if removed != CleanupBatchSize+12 || !repository.cutoff.Equal(wantCutoff) {
		t.Fatalf("removed=%d cutoff=%s, want %d and %s", removed, repository.cutoff, CleanupBatchSize+12, wantCutoff)
	}
	if len(repository.limits) != 2 || repository.limits[0] != CleanupBatchSize || repository.limits[1] != CleanupBatchSize {
		t.Fatalf("batch limits=%v, want two bounded batches of %d", repository.limits, CleanupBatchSize)
	}
}

func TestCleanup_GivenAFullBacklog_WhenRun_ThenItStopsAtTheMaximumBatchCount(t *testing.T) {
	counts := make([]int, cleanupMaxBatches)
	for index := range counts {
		counts[index] = CleanupBatchSize
	}
	repository := &retentionRepository{counts: counts}
	service := NewRetentionService(repository)

	removed, err := service.Cleanup(context.Background(), time.Hour)
	if err != nil || removed != cleanupMaxBatches*CleanupBatchSize || len(repository.limits) != cleanupMaxBatches {
		t.Fatalf("removed=%d batches=%d err=%v", removed, len(repository.limits), err)
	}
}

func TestCleanup_GivenInvalidRetentionOrRepositoryFailure_WhenRun_ThenItReturnsAnError(t *testing.T) {
	service := NewRetentionService(&retentionRepository{})
	if _, err := service.Cleanup(context.Background(), 0); err == nil {
		t.Fatal("expected zero retention to be rejected")
	}
	want := errors.New("database unavailable")
	service = NewRetentionService(&retentionRepository{err: want})
	if _, err := service.Cleanup(context.Background(), time.Hour); !errors.Is(err, want) {
		t.Fatalf("error=%v, want %v", err, want)
	}
}

func TestRunCleanup_GivenInvalidInterval_WhenStarted_ThenItReportsAndReturns(t *testing.T) {
	service := NewRetentionService(&retentionRepository{})
	reported := false
	service.RunCleanup(context.Background(), 0, time.Hour, func(_ int, err error) {
		reported = err != nil
	})
	if !reported {
		t.Fatal("invalid interval was not reported")
	}
}
