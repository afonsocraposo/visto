package tracking

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrFutureWatchTime = errors.New("watched_at cannot be in the future")

type Play struct {
	ID        string
	UserID    string
	MediaID   *string
	EpisodeID *string
	WatchedAt time.Time
	Source    string
}

type Repository interface {
	CreatePlay(context.Context, Play) error
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() string
}

func NewService(repository Repository, newID func() string) *Service {
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
