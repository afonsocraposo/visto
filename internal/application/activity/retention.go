package activity

import (
	"context"
	"fmt"
	"time"
)

const (
	CleanupBatchSize  = 500
	cleanupMaxBatches = 20
)

type RetentionRepository interface {
	CleanupActivityEvents(context.Context, time.Time, int) (int, error)
}

type RetentionService struct {
	repository RetentionRepository
	now        func() time.Time
}

func NewRetentionService(repository RetentionRepository) *RetentionService {
	return &RetentionService{repository: repository, now: time.Now}
}

// Cleanup removes activity events older than retention, measured from when
// Visto recorded them. A run is capped so a large backlog cannot hold SQLite
// writes for an unbounded time.
func (service *RetentionService) Cleanup(ctx context.Context, retention time.Duration) (int, error) {
	if retention <= 0 {
		return 0, fmt.Errorf("activity retention must be positive")
	}
	cutoff := service.now().UTC().Add(-retention)
	removed := 0
	for batch := 0; batch < cleanupMaxBatches; batch++ {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		count, err := service.repository.CleanupActivityEvents(ctx, cutoff, CleanupBatchSize)
		if err != nil {
			return removed, fmt.Errorf("clean activity events: %w", err)
		}
		removed += count
		if count < CleanupBatchSize {
			break
		}
	}
	return removed, nil
}

// RunCleanup starts an immediate cleanup and repeats it at interval until the
// application shuts down. report receives the number removed and any error.
func (service *RetentionService) RunCleanup(ctx context.Context, interval, retention time.Duration, report func(int, error)) {
	if interval <= 0 || retention <= 0 {
		if report != nil {
			report(0, fmt.Errorf("activity cleanup interval and retention must be positive"))
		}
		return
	}
	run := func() {
		removed, err := service.Cleanup(ctx, retention)
		if report != nil && (removed > 0 || err != nil) {
			report(removed, err)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
