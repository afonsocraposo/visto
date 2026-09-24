package profile

import (
	"context"
	"fmt"
	"time"
)

const (
	PrivateVisibility  = "private"
	InstanceVisibility = "instance"
)

type Settings struct {
	ActivityVisibility string `json:"activity_visibility"`
	Timezone           string `json:"timezone"`
}

type Repository interface {
	GetSettings(context.Context, string) (Settings, error)
	SetSettings(context.Context, string, Settings) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (service *Service) Get(ctx context.Context, userID string) (Settings, error) {
	if userID == "" {
		return Settings{}, fmt.Errorf("user is required")
	}
	return service.repository.GetSettings(ctx, userID)
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
