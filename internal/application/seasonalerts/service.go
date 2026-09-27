package seasonalerts

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("season is not in your library")
	ErrNotWatching  = errors.New("show must be Watching to subscribe")
	ErrAlreadyReady = errors.New("all episodes in this season are already available")
)

type State struct {
	Ready      bool `json:"ready"`
	Subscribed bool `json:"subscribed"`
}

type Delivery struct {
	UserID, SeasonID, Channel, SubscriptionID                  string
	EncryptedAppToken, EncryptedUserKey, EncryptedSubscription string
	ShowTitle                                                  string
	SeasonNumber, TMDBID                                       int
	AttemptCount                                               int
}

type Repository interface {
	SeasonAlertState(context.Context, string, string, time.Time) (State, error)
	SubscribeSeasonAlert(context.Context, string, string, time.Time) error
	CancelSeasonAlert(context.Context, string, string) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) State(ctx context.Context, userID, seasonID string) (State, error) {
	return s.repository.SeasonAlertState(ctx, userID, seasonID, time.Now().UTC())
}

func (s *Service) Subscribe(ctx context.Context, userID, seasonID string) error {
	return s.repository.SubscribeSeasonAlert(ctx, userID, seasonID, time.Now().UTC())
}

func (s *Service) Cancel(ctx context.Context, userID, seasonID string) error {
	return s.repository.CancelSeasonAlert(ctx, userID, seasonID)
}
