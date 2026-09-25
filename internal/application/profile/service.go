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
	ActivityVisibility  string `json:"activity_visibility"`
	Timezone            string `json:"timezone"`
	PushoverEnabled     bool   `json:"pushover_enabled"`
	HasPushoverAppToken bool   `json:"has_pushover_app_token"`
	HasPushoverUserKey  bool   `json:"has_pushover_user_key"`
	PushoverAvailable   bool   `json:"pushover_available"`
}

type Repository interface {
	GetSettings(context.Context, string) (Settings, error)
	SetSettings(context.Context, string, Settings) error
}

type SecretCipher interface {
	Encrypt(string) (string, error)
}

type pushoverSettingsRepository interface {
	GetPushoverSettings(context.Context, string) (enabled, hasAppToken, hasUserKey bool, err error)
	SetPushoverSettings(context.Context, string, *string, *string, bool) error
	ClearPushoverCredentials(context.Context, string) error
}

type PushoverConfig struct {
	Cipher SecretCipher
}

var (
	ErrPushoverUnavailable     = errors.New("secure storage for per-user Pushover credentials is unavailable")
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
	settings.PushoverAvailable = service.pushover.Cipher != nil
	if repository, ok := service.repository.(pushoverSettingsRepository); ok {
		settings.PushoverEnabled, settings.HasPushoverAppToken, settings.HasPushoverUserKey, err = repository.GetPushoverSettings(ctx, userID)
		if err != nil {
			return Settings{}, err
		}
	}
	return settings, nil
}

func (service *Service) UpdatePushover(ctx context.Context, userID string, enabled bool, appToken, userKey string) error {
	if userID == "" {
		return fmt.Errorf("user is required")
	}
	repository, ok := service.repository.(pushoverSettingsRepository)
	if !ok {
		return ErrPushoverUnavailable
	}
	var encryptedAppToken, encryptedUserKey *string
	appToken = strings.TrimSpace(appToken)
	userKey = strings.TrimSpace(userKey)
	if appToken != "" || userKey != "" {
		if service.pushover.Cipher == nil {
			return ErrPushoverUnavailable
		}
		if appToken != "" {
			if err := validatePushoverSecret("application token", appToken); err != nil {
				return err
			}
			ciphertext, err := service.pushover.Cipher.Encrypt(appToken)
			if err != nil {
				return fmt.Errorf("encrypt Pushover application token: %w", err)
			}
			encryptedAppToken = &ciphertext
		}
		if userKey != "" {
			if err := validatePushoverSecret("user key", userKey); err != nil {
				return err
			}
			ciphertext, err := service.pushover.Cipher.Encrypt(userKey)
			if err != nil {
				return fmt.Errorf("encrypt Pushover user key: %w", err)
			}
			encryptedUserKey = &ciphertext
		}
	}
	if enabled && service.pushover.Cipher == nil {
		return ErrPushoverUnavailable
	}
	if enabled && (encryptedAppToken == nil || encryptedUserKey == nil) {
		_, hasAppToken, hasUserKey, err := repository.GetPushoverSettings(ctx, userID)
		if err != nil {
			return err
		}
		if (encryptedAppToken == nil && !hasAppToken) || (encryptedUserKey == nil && !hasUserKey) {
			return fmt.Errorf("%w: add a Pushover application token and user key before enabling notifications", ErrInvalidPushoverSettings)
		}
	}
	return repository.SetPushoverSettings(ctx, userID, encryptedAppToken, encryptedUserKey, enabled)
}

func validatePushoverSecret(name, value string) error {
	if len(value) < 20 || len(value) > 80 || strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("%w: Pushover %s must be 20–80 characters without spaces", ErrInvalidPushoverSettings, name)
	}
	return nil
}

func (service *Service) ClearPushoverCredentials(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user is required")
	}
	repository, ok := service.repository.(pushoverSettingsRepository)
	if !ok {
		return ErrPushoverUnavailable
	}
	return repository.ClearPushoverCredentials(ctx, userID)
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
