package notifications

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Candidate struct {
	UserID           string
	EpisodeID        string
	EncryptedUserKey string
	ShowTitle        string
	EpisodeName      string
	SeasonNumber     int
	EpisodeNumber    int
}

type Repository interface {
	NotificationCandidates(context.Context, string, int) ([]Candidate, error)
	ClaimNotification(context.Context, string, string, time.Time) (bool, error)
	CompleteNotification(context.Context, string, string, time.Time) error
	FailNotification(context.Context, string, string, time.Time) error
}

type Sender interface {
	Send(context.Context, string, string, string) error
}

type Decryptor interface {
	Decrypt(string) (string, error)
}

type Service struct {
	repository Repository
	sender     Sender
	decryptor  Decryptor
	now        func() time.Time
}

func NewService(repository Repository, sender Sender, decryptor Decryptor) *Service {
	return &Service{repository: repository, sender: sender, decryptor: decryptor, now: time.Now}
}

func (service *Service) Dispatch(ctx context.Context) error {
	now := service.now().UTC()
	candidates, err := service.repository.NotificationCandidates(ctx, now.Format("2006-01-02"), 50)
	if err != nil {
		return fmt.Errorf("load new-episode notifications: %w", err)
	}
	var firstError error
	for _, candidate := range candidates {
		claimed, err := service.repository.ClaimNotification(ctx, candidate.UserID, candidate.EpisodeID, now)
		if err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("claim notification for %s: %w", candidate.EpisodeID, err)
			}
			continue
		}
		if !claimed {
			continue
		}
		userKey, err := service.decryptor.Decrypt(candidate.EncryptedUserKey)
		if err == nil {
			message := fmt.Sprintf("%s · S%02dE%02d is available", candidate.ShowTitle, candidate.SeasonNumber, candidate.EpisodeNumber)
			if candidate.EpisodeName != "" {
				message = fmt.Sprintf("%s · S%02dE%02d — %s is available", candidate.ShowTitle, candidate.SeasonNumber, candidate.EpisodeNumber, candidate.EpisodeName)
			}
			err = service.sender.Send(ctx, userKey, "New episode: "+candidate.ShowTitle, message)
		}
		if err != nil {
			if markErr := service.repository.FailNotification(ctx, candidate.UserID, candidate.EpisodeID, now); markErr != nil && firstError == nil {
				firstError = fmt.Errorf("record failed notification for %s: %w", candidate.EpisodeID, markErr)
			}
			if firstError == nil {
				firstError = fmt.Errorf("send notification for %s: %w", candidate.EpisodeID, err)
			}
			continue
		}
		if err := service.repository.CompleteNotification(ctx, candidate.UserID, candidate.EpisodeID, service.now().UTC()); err != nil && firstError == nil {
			firstError = fmt.Errorf("record sent notification for %s: %w", candidate.EpisodeID, err)
		}
	}
	return firstError
}

func (service *Service) Run(ctx context.Context, interval time.Duration, logger *log.Logger) {
	if interval <= 0 {
		if logger != nil {
			logger.Printf("Pushover notification interval must be positive")
		}
		return
	}
	if logger == nil {
		logger = log.Default()
	}
	if err := service.Dispatch(ctx); err != nil {
		logger.Printf("Pushover notification dispatch failed: %v", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := service.Dispatch(ctx); err != nil {
				logger.Printf("Pushover notification dispatch failed: %v", err)
			}
		}
	}
}
