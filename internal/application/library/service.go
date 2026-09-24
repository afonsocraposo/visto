package library

import (
	"context"
	"fmt"
	"github.com/afonsocosta/visto/internal/domain"
	"time"
)

type Item struct {
	UserID, MediaID    string
	Status             domain.LibraryStatus
	Rating             *int
	AddedAt, UpdatedAt time.Time
}
type Repository interface {
	UpsertItem(context.Context, Item) error
}
type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}
func (s *Service) Save(ctx context.Context, userID, mediaID string, status domain.LibraryStatus, rating *int) (Item, error) {
	if userID == "" || mediaID == "" {
		return Item{}, fmt.Errorf("user and media are required")
	}
	if status != domain.WatchlistStatus && status != domain.WatchingStatus && status != domain.PausedStatus && status != domain.DroppedStatus {
		return Item{}, fmt.Errorf("invalid library status")
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return Item{}, fmt.Errorf("rating must be from 1 to 5")
	}
	now := s.now().UTC()
	item := Item{UserID: userID, MediaID: mediaID, Status: status, Rating: rating, AddedAt: now, UpdatedAt: now}
	if err := s.repository.UpsertItem(ctx, item); err != nil {
		return Item{}, err
	}
	return item, nil
}
