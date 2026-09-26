package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type WebCandidate struct {
	UserID, EpisodeID, SubscriptionID, EncryptedSubscription, ShowTitle, EpisodeName string
	SeasonNumber, EpisodeNumber, TMDBID                                              int
}
type WebRepository interface {
	WebPushCandidates(context.Context, time.Time, int) ([]WebCandidate, error)
	ClaimWebPush(context.Context, string, string, string, time.Time) (bool, error)
	CompleteWebPush(context.Context, string, string, string, time.Time) error
	FailWebPush(context.Context, string, string, string, time.Time) error
	DeletePushSubscription(context.Context, string, string) error
}
type WebSender interface {
	SendPush(context.Context, string, []byte) (expired bool, err error)
}

type WebService struct {
	Repository WebRepository
	Sender     WebSender
	Decryptor  Decryptor
}

func (s *WebService) Dispatch(ctx context.Context) error {
	now := time.Now().UTC()
	candidates, err := s.Repository.WebPushCandidates(ctx, now, 50)
	if err != nil {
		return err
	}
	var first error
	for _, c := range candidates {
		claimed, err := s.Repository.ClaimWebPush(ctx, c.UserID, c.EpisodeID, c.SubscriptionID, now)
		if err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		if !claimed {
			continue
		}
		body := fmt.Sprintf("%s · S%02dE%02d is available", c.ShowTitle, c.SeasonNumber, c.EpisodeNumber)
		if c.EpisodeName != "" {
			body = fmt.Sprintf("%s · S%02dE%02d — %s is available", c.ShowTitle, c.SeasonNumber, c.EpisodeNumber, c.EpisodeName)
		}
		payload, _ := json.Marshal(map[string]string{"title": "New episode: " + c.ShowTitle, "body": body, "url": fmt.Sprintf("/media/tv/%d", c.TMDBID), "user_id": c.UserID})
		subscription, decryptErr := s.Decryptor.Decrypt(c.EncryptedSubscription)
		expired := false
		if decryptErr == nil {
			expired, err = s.Sender.SendPush(ctx, subscription, payload)
		} else {
			err = decryptErr
		}
		if expired {
			if removeErr := s.Repository.DeletePushSubscription(ctx, c.UserID, c.SubscriptionID); removeErr != nil && first == nil {
				first = removeErr
			}
			continue
		}
		if err != nil {
			if markErr := s.Repository.FailWebPush(ctx, c.UserID, c.EpisodeID, c.SubscriptionID, now); markErr != nil && first == nil {
				first = markErr
			}
			if first == nil {
				first = err
			}
			continue
		}
		if err := s.Repository.CompleteWebPush(ctx, c.UserID, c.EpisodeID, c.SubscriptionID, time.Now().UTC()); err != nil && first == nil {
			first = err
		}
	}
	return first
}
