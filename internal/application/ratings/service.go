package ratings

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

const freshnessWindow = 7 * 24 * time.Hour

type Scores struct {
	IMDbID         string    `json:"imdb_id,omitempty"`
	IMDb           *float64  `json:"imdb,omitempty"`
	RottenTomatoes *int      `json:"rotten_tomatoes,omitempty"`
	FetchedAt      time.Time `json:"fetched_at,omitzero"`
}

type Result struct {
	Scores
	Enabled bool `json:"enabled"`
}

type Repository interface {
	GetExternalRatings(context.Context, domain.MediaType, int64) (Scores, bool, error)
	SaveExternalRatings(context.Context, domain.MediaType, int64, Scores) error
	ExternalRatingsDue(context.Context, time.Time, int) ([]Title, error)
}

type Title struct {
	Type   domain.MediaType
	TMDBID int64
}

type IMDbResolver interface {
	IMDbID(context.Context, domain.MediaType, int64) (string, error)
}

type Provider interface {
	Ratings(context.Context, string) (Scores, error)
}

type Service struct {
	repository Repository
	resolver   IMDbResolver
	provider   Provider
	// ponytail: serialize cold lookups for a small self-hosted instance; use per-title
	// request coalescing if concurrent provider latency becomes a problem.
	mu         sync.Mutex
	retryAfter time.Time
	day        string
	requests   int
}

func New(repository Repository, resolver IMDbResolver, provider Provider) *Service {
	return &Service{repository: repository, resolver: resolver, provider: provider}
}

func (s *Service) Get(ctx context.Context, mediaType domain.MediaType, tmdbID int64) (Result, error) {
	return s.get(ctx, mediaType, tmdbID, false)
}

func (s *Service) get(ctx context.Context, mediaType domain.MediaType, tmdbID int64, refresh bool) (Result, error) {
	if (mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType) || tmdbID <= 0 {
		return Result{}, fmt.Errorf("valid media type and TMDB ID are required")
	}
	if s == nil || s.provider == nil || s.resolver == nil {
		return Result{}, nil
	}
	cached, found, err := s.repository.GetExternalRatings(ctx, mediaType, tmdbID)
	if err != nil {
		return Result{}, err
	}
	result := Result{Scores: cached, Enabled: true}
	if found && (!refresh || time.Since(cached.FetchedAt) < freshnessWindow) {
		return result, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A concurrent request may have filled the durable cache while we waited.
	cached, found, err = s.repository.GetExternalRatings(ctx, mediaType, tmdbID)
	if err != nil {
		return Result{}, err
	}
	result.Scores = cached
	now := time.Now().UTC()
	if found && (!refresh || now.Sub(cached.FetchedAt) < freshnessWindow) {
		return result, nil
	}
	if s.day != now.Format(time.DateOnly) {
		s.day, s.requests = now.Format(time.DateOnly), 0
	}
	// Leave headroom below the free tier's 1,000 daily requests. OMDb still
	// enforces the key's quota across other apps and server restarts.
	if now.Before(s.retryAfter) || s.requests >= 900 {
		return result, nil
	}
	imdbID := cached.IMDbID
	if imdbID == "" {
		imdbID, err = s.resolver.IMDbID(ctx, mediaType, tmdbID)
		if err != nil {
			s.retryAfter = now.Add(10 * time.Minute)
			return result, nil
		}
	}
	scores := Scores{IMDbID: imdbID}
	if imdbID != "" {
		s.requests++
		scores, err = s.provider.Ratings(ctx, imdbID)
		if err != nil {
			s.retryAfter = now.Add(10 * time.Minute)
			return result, nil
		}
		scores.IMDbID = imdbID
	}
	scores.FetchedAt = now
	if err := s.repository.SaveExternalRatings(ctx, mediaType, tmdbID, scores); err != nil {
		return Result{}, err
	}
	return Result{Scores: scores, Enabled: true}, nil
}

// Refresh updates a bounded batch of previously requested scores. Page views
// always reuse stored results; only the first lookup contacts the providers.
func (s *Service) Refresh(ctx context.Context) error {
	if s == nil || s.provider == nil || s.resolver == nil {
		return nil
	}
	titles, err := s.repository.ExternalRatingsDue(ctx, time.Now().UTC().Add(-freshnessWindow), 20)
	if err != nil {
		return err
	}
	for _, title := range titles {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := s.get(ctx, title.Type, title.TMDBID, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) RunRefresher(ctx context.Context) {
	if s == nil || s.provider == nil || s.resolver == nil {
		return
	}
	run := func() {
		if err := s.Refresh(ctx); err != nil && ctx.Err() == nil {
			log.Printf("ratings refresh failed: %v", err)
		}
	}
	run()
	ticker := time.NewTicker(time.Hour)
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
