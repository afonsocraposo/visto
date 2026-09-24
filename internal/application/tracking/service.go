package tracking

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

var (
	ErrFutureWatchTime = errors.New("watched_at cannot be in the future")
	ErrPlayNotFound    = errors.New("play not found")
)

type Play struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	MediaID   *string   `json:"media_id"`
	EpisodeID *string   `json:"episode_id"`
	WatchedAt time.Time `json:"watched_at"`
	Source    string    `json:"source"`
}

type HistoryEntry struct {
	Play         Play   `json:"play"`
	Title        string `json:"title"`
	EpisodeLabel string `json:"episode_label,omitempty"`
}

type Repository interface {
	CreatePlay(context.Context, Play) error
	CreateBulkPlays(context.Context, []Play) error
	UpdatePlay(context.Context, string, string, time.Time) error
	DeletePlay(context.Context, string, string) error
}

type historyRepository interface {
	ListPlays(context.Context, string, int) ([]HistoryEntry, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() string
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now, newID: newID}
}

func NewServiceWithID(repository Repository, newID func() string) *Service {
	return &Service{repository: repository, now: time.Now, newID: newID}
}

func (service *Service) Record(ctx context.Context, userID string, mediaID, episodeID *string, watchedAt time.Time, source string) (Play, error) {
	if userID == "" {
		return Play{}, fmt.Errorf("user is required")
	}
	if (mediaID == nil && episodeID == nil) || (mediaID != nil && episodeID != nil) {
		return Play{}, fmt.Errorf("a play must identify exactly one movie or episode")
	}
	now := service.now().UTC()
	if watchedAt.IsZero() {
		watchedAt = now
	}
	if watchedAt.After(now) {
		return Play{}, ErrFutureWatchTime
	}
	if source == "" {
		source = "web"
	}
	play := Play{ID: service.newID(), UserID: userID, MediaID: mediaID, EpisodeID: episodeID, WatchedAt: watchedAt.UTC(), Source: source}
	if err := service.repository.CreatePlay(ctx, play); err != nil {
		return Play{}, err
	}
	return play, nil
}

func (service *Service) RecordEpisodes(ctx context.Context, userID string, episodeIDs []string, watchedAt time.Time, source string) ([]Play, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	if len(episodeIDs) == 0 || len(episodeIDs) > 100 {
		return nil, fmt.Errorf("bulk action must include 1 to 100 episodes")
	}
	now := service.now().UTC()
	if watchedAt.IsZero() {
		watchedAt = now
	}
	if watchedAt.After(now) {
		return nil, ErrFutureWatchTime
	}
	if source == "" {
		source = "web"
	}
	seen := map[string]bool{}
	plays := make([]Play, 0, len(episodeIDs))
	for _, episodeID := range episodeIDs {
		if episodeID == "" || seen[episodeID] {
			return nil, fmt.Errorf("episode IDs must be non-empty and unique")
		}
		seen[episodeID] = true
		id := episodeID
		plays = append(plays, Play{ID: service.newID(), UserID: userID, EpisodeID: &id, WatchedAt: watchedAt.UTC(), Source: source})
	}
	if err := service.repository.CreateBulkPlays(ctx, plays); err != nil {
		return nil, err
	}
	return plays, nil
}

func (service *Service) Correct(ctx context.Context, userID, playID string, watchedAt time.Time) error {
	if userID == "" || playID == "" {
		return fmt.Errorf("user and play are required")
	}
	if watchedAt.IsZero() {
		return fmt.Errorf("watched_at is required")
	}
	if watchedAt.After(service.now().UTC()) {
		return ErrFutureWatchTime
	}
	if err := service.repository.UpdatePlay(ctx, userID, playID, watchedAt.UTC()); err != nil {
		return err
	}
	return nil
}

func (service *Service) Remove(ctx context.Context, userID, playID string) error {
	if userID == "" || playID == "" {
		return fmt.Errorf("user and play are required")
	}
	return service.repository.DeletePlay(ctx, userID, playID)
}

func (service *Service) History(ctx context.Context, userID string, limit int) ([]HistoryEntry, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		return nil, fmt.Errorf("limit must not exceed 500")
	}
	repository, ok := service.repository.(historyRepository)
	if !ok {
		return nil, fmt.Errorf("history storage is not configured")
	}
	return repository.ListPlays(ctx, userID, limit)
}

func newID() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("generate play ID: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}
