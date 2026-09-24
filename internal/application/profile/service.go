package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	PrivateVisibility  = "private"
	InstanceVisibility = "instance"
)

type Settings struct {
	ActivityVisibility string `json:"activity_visibility"`
	Timezone           string `json:"timezone"`
	PushoverEnabled    bool   `json:"pushover_enabled"`
	HasPushoverKey     bool   `json:"has_pushover_key"`
	PushoverAvailable  bool   `json:"pushover_available"`
}

type Repository interface {
	GetSettings(context.Context, string) (Settings, error)
	SetSettings(context.Context, string, Settings) error
}

type SecretCipher interface {
	Encrypt(string) (string, error)
}

type pushoverSettingsRepository interface {
	GetPushoverSettings(context.Context, string) (enabled, hasKey bool, err error)
	SetPushoverSettings(context.Context, string, *string, bool) error
	ClearPushoverKey(context.Context, string) error
}

type PushoverConfig struct {
	Available bool
	Cipher    SecretCipher
}

var (
	ErrPushoverUnavailable     = errors.New("Pushover is not configured for this instance")
	ErrInvalidPushoverSettings = errors.New("invalid Pushover settings")
)

type Service struct {
	repository Repository
	pushover   PushoverConfig
}

func NewService(repository Repository, configs ...PushoverConfig) *Service {
	service := &Service{repository: repository}
	if len(configs) > 0 {
		service.pushover = configs[0]
	}
	return service
}

func (service *Service) Get(ctx context.Context, userID string) (Settings, error) {
	if userID == "" {
		return Settings{}, fmt.Errorf("user is required")
	}
	settings, err := service.repository.GetSettings(ctx, userID)
	if err != nil {
		return Settings{}, err
	}
	settings.PushoverAvailable = service.pushover.Available && service.pushover.Cipher != nil
	if repository, ok := service.repository.(pushoverSettingsRepository); ok {
		settings.PushoverEnabled, settings.HasPushoverKey, err = repository.GetPushoverSettings(ctx, userID)
		if err != nil {
			return Settings{}, err
		}
	}
	return settings, nil
}

func (service *Service) UpdatePushover(ctx context.Context, userID string, enabled bool, userKey string) error {
	if userID == "" {
		return fmt.Errorf("user is required")
	}
	repository, ok := service.repository.(pushoverSettingsRepository)
	if !ok {
		return ErrPushoverUnavailable
	}
	var encryptedKey *string
	userKey = strings.TrimSpace(userKey)
	if userKey != "" {
		if !service.pushover.Available || service.pushover.Cipher == nil {
			return ErrPushoverUnavailable
		}
		if len(userKey) < 20 || len(userKey) > 80 || strings.ContainsAny(userKey, " \t\r\n") {
			return fmt.Errorf("%w: user key must be 20–80 characters without spaces", ErrInvalidPushoverSettings)
		}
		ciphertext, err := service.pushover.Cipher.Encrypt(userKey)
		if err != nil {
			return fmt.Errorf("encrypt Pushover user key: %w", err)
		}
		encryptedKey = &ciphertext
	}
	if enabled && (!service.pushover.Available || service.pushover.Cipher == nil) {
		return ErrPushoverUnavailable
	}
	if enabled && encryptedKey == nil {
		_, hasKey, err := repository.GetPushoverSettings(ctx, userID)
		if err != nil {
			return err
		}
		if !hasKey {
			return fmt.Errorf("%w: add a Pushover user key before enabling notifications", ErrInvalidPushoverSettings)
		}
	}
	return repository.SetPushoverSettings(ctx, userID, encryptedKey, enabled)
}

func (service *Service) ClearPushoverKey(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user is required")
	}
	repository, ok := service.repository.(pushoverSettingsRepository)
	if !ok {
		return ErrPushoverUnavailable
	}
	return repository.ClearPushoverKey(ctx, userID)
}

func (service *Service) SetActivityVisibility(ctx context.Context, userID, visibility string) error {
	if userID == "" {
		return fmt.Errorf("user is required")
	}
	if visibility != PrivateVisibility && visibility != InstanceVisibility {
		return fmt.Errorf("invalid activity visibility")
	}
	settings, err := service.repository.GetSettings(ctx, userID)
	if err != nil {
		return err
	}
	settings.ActivityVisibility = visibility
	return service.repository.SetSettings(ctx, userID, settings)
}

func (service *Service) Update(ctx context.Context, userID string, settings Settings) error {
	if userID == "" {
		return fmt.Errorf("user is required")
	}
	if settings.ActivityVisibility != "" && settings.ActivityVisibility != PrivateVisibility && settings.ActivityVisibility != InstanceVisibility {
		return fmt.Errorf("invalid activity visibility")
	}
	if settings.Timezone != "" {
		if _, err := time.LoadLocation(settings.Timezone); err != nil {
			return fmt.Errorf("invalid timezone")
		}
	}
	current, err := service.repository.GetSettings(ctx, userID)
	if err != nil {
		return err
	}
	if settings.ActivityVisibility == "" {
		settings.ActivityVisibility = current.ActivityVisibility
	}
	if settings.Timezone == "" {
		settings.Timezone = current.Timezone
	}
	return service.repository.SetSettings(ctx, userID, settings)
}
