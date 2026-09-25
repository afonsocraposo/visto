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
