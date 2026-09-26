package notifications

import (
	"context"
	"errors"
	"testing"
	"time"
)

type webRepoFake struct {
	candidates            []WebCandidate
	claimed               map[string]bool
	sent, failed, removed int
}

func (r *webRepoFake) WebPushCandidates(context.Context, time.Time, int) ([]WebCandidate, error) {
	return r.candidates, nil
}
func (r *webRepoFake) ClaimWebPush(_ context.Context, _, episode, device string, _ time.Time) (bool, error) {
	if r.claimed == nil {
		r.claimed = map[string]bool{}
	}
	key := episode + device
	if r.claimed[key] {
		return false, nil
	}
	r.claimed[key] = true
	return true, nil
}
func (r *webRepoFake) CompleteWebPush(context.Context, string, string, string, time.Time) error {
	r.sent++
	return nil
}
func (r *webRepoFake) FailWebPush(context.Context, string, string, string, time.Time) error {
	r.failed++
	return nil
}
func (r *webRepoFake) DeletePushSubscription(context.Context, string, string) error {
	r.removed++
	return nil
}

type webSenderFake struct {
	calls   int
	expired bool
	err     error
}

func (s *webSenderFake) SendPush(context.Context, string, []byte) (bool, error) {
	s.calls++
	return s.expired, s.err
}
func TestWebPushPerDeviceAndExpiredRemoval(t *testing.T) {
	r := &webRepoFake{candidates: []WebCandidate{{UserID: "1", EpisodeID: "episode", SubscriptionID: "phone", EncryptedSubscription: "one", ShowTitle: "Show", SeasonNumber: 1, EpisodeNumber: 1, TMDBID: 42}, {UserID: "1", EpisodeID: "episode", SubscriptionID: "laptop", EncryptedSubscription: "two", ShowTitle: "Show", SeasonNumber: 1, EpisodeNumber: 1, TMDBID: 42}}}
	sender := &webSenderFake{}
	service := &WebService{Repository: r, Sender: sender, Decryptor: notificationDecryptorFake{}}
	if err := service.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 2 || r.sent != 2 {
		t.Fatalf("calls=%d sent=%d", sender.calls, r.sent)
	}
	sender.expired = true
	r.claimed = map[string]bool{}
	if err := service.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.removed != 2 {
		t.Fatalf("removed=%d", r.removed)
	}
	sender.expired = false
	sender.err = errors.New("provider failure")
	r.claimed = map[string]bool{}
	if err := service.Dispatch(context.Background()); err == nil || r.failed != 2 {
		t.Fatalf("failed=%d err=%v", r.failed, err)
	}
}
