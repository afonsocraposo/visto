package profile

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type profileRepositoryFake struct {
	settings Settings
	enabled  bool
	appToken string
	userKey  string
}

func (repository *profileRepositoryFake) GetSettings(context.Context, string) (Settings, error) {
	return repository.settings, nil
}
func (repository *profileRepositoryFake) SetSettings(_ context.Context, _ string, settings Settings) error {
	repository.settings = settings
	return nil
}
func (repository *profileRepositoryFake) GetPushoverSettings(context.Context, string) (bool, bool, bool, error) {
	return repository.enabled, repository.appToken != "", repository.userKey != "", nil
}
func (repository *profileRepositoryFake) GetPushoverCredentials(context.Context, string) (string, string, error) {
	return repository.appToken, repository.userKey, nil
}
func (repository *profileRepositoryFake) SetPushoverSettings(_ context.Context, _ string, appToken, userKey *string, enabled bool) error {
	if appToken != nil {
		repository.appToken = *appToken
	}
	if userKey != nil {
		repository.userKey = *userKey
	}
	repository.enabled = enabled
	return nil
}
func (repository *profileRepositoryFake) ClearPushoverCredentials(context.Context, string) error {
	repository.appToken = ""
	repository.userKey = ""
	repository.enabled = false
	return nil
}

type profileCipherFake struct{}

func (profileCipherFake) Encrypt(value string) (string, error) { return "encrypted:" + value, nil }
func (profileCipherFake) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "encrypted:"), nil
}

func TestUpdatePushoverEncryptsEachUsersCredentialsAndRequiresBothBeforeEnable(t *testing.T) {
	repository := &profileRepositoryFake{}
	service := NewService(repository, PushoverConfig{Cipher: profileCipherFake{}})
	if err := service.UpdatePushover(context.Background(), "user-1", true, "", ""); !errors.Is(err, ErrInvalidPushoverSettings) {
		t.Fatalf("expected missing-credentials error, got %v", err)
	}
	secret := strings.Repeat("x", 32)
	if err := service.UpdatePushover(context.Background(), "user-1", true, secret, secret); err != nil {
		t.Fatal(err)
	}
	if !repository.enabled || repository.appToken != "encrypted:"+secret || repository.userKey != "encrypted:"+secret {
		t.Fatalf("stored enabled=%v app token=%q user key=%q", repository.enabled, repository.appToken, repository.userKey)
	}
}

func TestUpdatePushoverRejectsUnavailableAndMalformedKeys(t *testing.T) {
	repository := &profileRepositoryFake{}
	unavailable := NewService(repository)
	if err := unavailable.UpdatePushover(context.Background(), "user-1", true, "", ""); !errors.Is(err, ErrPushoverUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
	service := NewService(repository, PushoverConfig{Cipher: profileCipherFake{}})
	if err := service.UpdatePushover(context.Background(), "user-1", false, "short", ""); !errors.Is(err, ErrInvalidPushoverSettings) {
		t.Fatalf("expected malformed key error, got %v", err)
	}
}

type profileSenderFake struct {
	calls                   int
	token, key, title, body string
	err                     error
}

func (s *profileSenderFake) Send(_ context.Context, token, key, title, body string) error {
	s.calls++
	s.token, s.key, s.title, s.body = token, key, title, body
	return s.err
}
func TestPushoverTestUsesSavedCredentialsWithoutEnablingScheduledAlerts(t *testing.T) {
	repo := &profileRepositoryFake{}
	sender := &profileSenderFake{}
	service := NewService(repo, PushoverConfig{Cipher: profileCipherFake{}, Sender: sender})
	if err := service.TestPushover(context.Background(), "user-1"); !errors.Is(err, ErrPushoverNotConfigured) {
		t.Fatalf("missing credentials: %v", err)
	}
	secret := strings.Repeat("x", 32)
	if err := service.UpdatePushover(context.Background(), "user-1", false, secret, secret); err != nil {
		t.Fatal(err)
	}
	if err := service.TestPushover(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 || sender.token != secret || sender.key != secret || sender.title != "Visto test notification" || repo.enabled {
		t.Fatalf("send=%+v enabled=%t", sender, repo.enabled)
	}
	sender.err = errors.New("provider rejected")
	if err := service.TestPushover(context.Background(), "user-1"); err == nil {
		t.Fatal("expected provider error")
	}
}
