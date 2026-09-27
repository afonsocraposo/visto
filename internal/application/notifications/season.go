package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/application/seasonalerts"
)

type SeasonRepository interface {
	MarkReadySeasonAlerts(context.Context, time.Time, bool, bool, int) error
	DueSeasonDeliveries(context.Context, time.Time, int) ([]seasonalerts.Delivery, error)
	ClaimSeasonDelivery(context.Context, seasonalerts.Delivery, time.Time) (bool, error)
	FinishSeasonDelivery(context.Context, seasonalerts.Delivery, time.Time, bool, *time.Time) error
	RemoveSeasonWebDelivery(context.Context, seasonalerts.Delivery) error
}

type SeasonService struct {
	Repository SeasonRepository
	Pushover   Sender
	Web        WebSender
	Decryptor  Decryptor
	Now        func() time.Time
}

func (s *SeasonService) Dispatch(ctx context.Context) error {
	if s == nil || s.Repository == nil {
		return nil
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	if err := s.Repository.MarkReadySeasonAlerts(ctx, now, s.Pushover != nil, s.Web != nil, 50); err != nil {
		return err
	}
	deliveries, err := s.Repository.DueSeasonDeliveries(ctx, now, 50)
	if err != nil {
		return err
	}
	var first error
	for _, d := range deliveries {
		if d.Channel == "pushover" && s.Pushover == nil || d.Channel == "web_push" && s.Web == nil {
			continue
		}
		claimed, err := s.Repository.ClaimSeasonDelivery(ctx, d, now)
		if err != nil {
			first = errors.Join(first, err)
			continue
		}
		if !claimed {
			continue
		}
		title := fmt.Sprintf("Season ready: %s", d.ShowTitle)
		body := fmt.Sprintf("All episodes of %s season %d are available.", d.ShowTitle, d.SeasonNumber)
		expired := false
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
				payload, _ := json.Marshal(map[string]string{
					"title": title, "body": body,
					"url":     fmt.Sprintf("/media/tv/%d?season=%d", d.TMDBID, d.SeasonNumber),
					"user_id": d.UserID,
				})
				expired, err = s.Web.SendPush(ctx, subscription, payload)
			}
		}
		if expired {
			first = errors.Join(first, s.Repository.RemoveSeasonWebDelivery(ctx, d))
			continue
		}
		var next *time.Time
		if err != nil {
			if delay := RetryDelay(d.AttemptCount + 1); delay > 0 {
				retry := now.Add(delay)
				next = &retry
			}
		}
		first = errors.Join(first, s.Repository.FinishSeasonDelivery(ctx, d, now, err != nil, next))
		first = errors.Join(first, err)
	}
	return first
}
