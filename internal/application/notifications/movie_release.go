package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type MovieReleaseDelivery struct {
	UserID, MediaID, Channel, SubscriptionID string
	EncryptedAppToken, EncryptedUserKey      string
	EncryptedSubscription                    string
	Title                                    string
	TMDBID                                   int64
	AttemptCount                             int
}

type MovieReleaseRepository interface {
	PrepareMovieReleaseDeliveries(context.Context, time.Time, bool, bool, int) error
	MovieReleaseDeliveries(context.Context, time.Time, int) ([]MovieReleaseDelivery, error)
	ClaimMovieReleaseDelivery(context.Context, MovieReleaseDelivery, time.Time) (bool, error)
	FinishMovieReleaseDelivery(context.Context, MovieReleaseDelivery, time.Time, bool, *time.Time) error
	RemoveMovieReleaseWebDelivery(context.Context, MovieReleaseDelivery) error
}

type MovieReleaseService struct {
	Repository MovieReleaseRepository
	Pushover   Sender
	Web        WebSender
	Decryptor  Decryptor
	Now        func() time.Time
}

func (s *MovieReleaseService) Dispatch(ctx context.Context) error {
	if s == nil || s.Repository == nil {
		return nil
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	if err := s.Repository.PrepareMovieReleaseDeliveries(ctx, now, s.Pushover != nil, s.Web != nil, 50); err != nil {
		return err
	}
	deliveries, err := s.Repository.MovieReleaseDeliveries(ctx, now, 50)
	if err != nil {
		return err
	}
	var first error
	for _, d := range deliveries {
		if d.Channel == "pushover" && s.Pushover == nil || d.Channel == "web_push" && s.Web == nil {
			continue
		}
		claimed, err := s.Repository.ClaimMovieReleaseDelivery(ctx, d, now)
		if err != nil {
			first = errors.Join(first, err)
			continue
		}
		if !claimed {
			continue
		}
		title, body := "Now available: "+d.Title, d.Title+" is out today."
		var expired bool
		if d.Channel == "pushover" {
			var token, key string
			token, err = s.Decryptor.Decrypt(d.EncryptedAppToken)
			if err == nil {
				key, err = s.Decryptor.Decrypt(d.EncryptedUserKey)
			}
			if err == nil {
				err = s.Pushover.Send(ctx, token, key, title, body)
			}
		} else {
			var subscription string
			subscription, err = s.Decryptor.Decrypt(d.EncryptedSubscription)
			if err == nil {
				payload, _ := json.Marshal(map[string]string{"title": title, "body": body, "url": fmt.Sprintf("/media/movie/%d", d.TMDBID), "user_id": d.UserID})
				expired, err = s.Web.SendPush(ctx, subscription, payload)
			}
		}
		if expired {
			first = errors.Join(first, s.Repository.RemoveMovieReleaseWebDelivery(ctx, d))
			continue
		}
		var next *time.Time
		if err != nil {
			if delay := RetryDelay(d.AttemptCount + 1); delay > 0 {
				retry := now.Add(delay)
				next = &retry
			}
		}
		first = errors.Join(first, s.Repository.FinishMovieReleaseDelivery(ctx, d, now, err != nil, next), err)
	}
	return first
}
