package notifications

import (
	"context"
	"errors"
	"testing"
	"time"
)

type notificationRepositoryFake struct {
	candidates []Candidate
	claimed    map[string]bool
	completed  int
	failed     int
}

func (repository *notificationRepositoryFake) NotificationCandidates(context.Context, string, int) ([]Candidate, error) {
	return repository.candidates, nil
}
func (repository *notificationRepositoryFake) ClaimNotification(_ context.Context, userID, episodeID string, _ time.Time) (bool, error) {
	if repository.claimed == nil {
		repository.claimed = map[string]bool{}
	}
	key := userID + ":" + episodeID
	if repository.claimed[key] {
		return false, nil
	}
	repository.claimed[key] = true
	return true, nil
}
func (repository *notificationRepositoryFake) CompleteNotification(context.Context, string, string, time.Time) error {
	repository.completed++
	return nil
}
func (repository *notificationRepositoryFake) FailNotification(context.Context, string, string, time.Time) error {
	repository.failed++
	return nil
}

type notificationSenderFake struct {
	userKey string
	title   string
	body    string
	err     error
	calls   int
}

func (sender *notificationSenderFake) Send(_ context.Context, userKey, title, body string) error {
	sender.calls++
	sender.userKey, sender.title, sender.body = userKey, title, body
	return sender.err
}

type notificationDecryptorFake struct{}

func (notificationDecryptorFake) Decrypt(value string) (string, error) {
	return "decrypted:" + value, nil
}

func TestDispatchSendsClaimedNewEpisodeOnce(t *testing.T) {
	repository := &notificationRepositoryFake{candidates: []Candidate{{
		UserID: "user-1", EpisodeID: "episode-1", EncryptedUserKey: "ciphertext", ShowTitle: "Severance", EpisodeName: "Hello, Ms. Cobel", SeasonNumber: 2, EpisodeNumber: 3,
	}}}
	sender := &notificationSenderFake{}
	service := NewService(repository, sender, notificationDecryptorFake{})
	service.now = func() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) }
	if err := service.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 || repository.completed != 1 || repository.failed != 0 {
		t.Fatalf("calls=%d completed=%d failed=%d", sender.calls, repository.completed, repository.failed)
	}
	if sender.userKey != "decrypted:ciphertext" || sender.title != "New episode: Severance" || sender.body != "Severance · S02E03 — Hello, Ms. Cobel is available" {
		t.Fatalf("unexpected Pushover message: %#v", sender)
	}
}

func TestDispatchRecordsFailedDeliveryAndContinues(t *testing.T) {
	repository := &notificationRepositoryFake{candidates: []Candidate{{UserID: "user-1", EpisodeID: "episode-1", EncryptedUserKey: "ciphertext", ShowTitle: "Severance", SeasonNumber: 2, EpisodeNumber: 3}}}
	sender := &notificationSenderFake{err: errors.New("provider unavailable")}
	service := NewService(repository, sender, notificationDecryptorFake{})
	if err := service.Dispatch(context.Background()); err == nil {
		t.Fatal("expected send failure")
	}
	if sender.calls != 1 || repository.failed != 1 || repository.completed != 0 {
		t.Fatalf("calls=%d completed=%d failed=%d", sender.calls, repository.completed, repository.failed)
	}
}
