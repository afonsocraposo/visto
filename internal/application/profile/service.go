package profile

import (
	"context"
	"fmt"
)

const (
	PrivateVisibility  = "private"
	InstanceVisibility = "instance"
)

type Settings struct {
	ActivityVisibility string `json:"activity_visibility"`
}

type Repository interface {
	GetSettings(context.Context, string) (Settings, error)
	SetActivityVisibility(context.Context, string, string) error
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
	return service.repository.SetActivityVisibility(ctx, userID, visibility)
}
