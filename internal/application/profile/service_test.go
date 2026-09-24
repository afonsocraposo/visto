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
	key      string
}

func (repository *profileRepositoryFake) GetSettings(context.Context, string) (Settings, error) {
	return repository.settings, nil
}
func (repository *profileRepositoryFake) SetSettings(_ context.Context, _ string, settings Settings) error {
	repository.settings = settings
	return nil
}
func (repository *profileRepositoryFake) GetPushoverSettings(context.Context, string) (bool, bool, error) {
	return repository.enabled, repository.key != "", nil
}
func (repository *profileRepositoryFake) SetPushoverSettings(_ context.Context, _ string, key *string, enabled bool) error {
	if key != nil {
		repository.key = *key
	}
	repository.enabled = enabled
	return nil
}
func (repository *profileRepositoryFake) ClearPushoverKey(context.Context, string) error {
	repository.key = ""
	repository.enabled = false
	return nil
}

type profileCipherFake struct{}

func (profileCipherFake) Encrypt(value string) (string, error) { return "encrypted:" + value, nil }

func TestUpdatePushoverEncryptsUserKeyAndRequiresKeyBeforeEnable(t *testing.T) {
	repository := &profileRepositoryFake{}
	service := NewService(repository, PushoverConfig{Available: true, Cipher: profileCipherFake{}})
	if err := service.UpdatePushover(context.Background(), "user-1", true, ""); !errors.Is(err, ErrInvalidPushoverSettings) {
		t.Fatalf("expected missing-key error, got %v", err)
	}
	if err := service.UpdatePushover(context.Background(), "user-1", true, strings.Repeat("x", 32)); err != nil {
		t.Fatal(err)
	}
	if !repository.enabled || repository.key != "encrypted:"+strings.Repeat("x", 32) {
		t.Fatalf("stored enabled=%v key=%q", repository.enabled, repository.key)
	}
}

func TestUpdatePushoverRejectsUnavailableAndMalformedKeys(t *testing.T) {
	repository := &profileRepositoryFake{}
	unavailable := NewService(repository)
	if err := unavailable.UpdatePushover(context.Background(), "user-1", true, ""); !errors.Is(err, ErrPushoverUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
	service := NewService(repository, PushoverConfig{Available: true, Cipher: profileCipherFake{}})
	if err := service.UpdatePushover(context.Background(), "user-1", false, "short"); !errors.Is(err, ErrInvalidPushoverSettings) {
		t.Fatalf("expected malformed key error, got %v", err)
	}
}
