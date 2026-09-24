package export

import (
	"context"
	"fmt"
	"time"
)

type LibraryItem struct {
	MediaID string `json:"media_id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Rating  *int   `json:"rating"`
}

type Play struct {
	ID        string    `json:"id"`
	MediaID   *string   `json:"media_id"`
	EpisodeID *string   `json:"episode_id"`
	WatchedAt time.Time `json:"watched_at"`
}

type Data struct {
	ExportedAt time.Time     `json:"exported_at"`
	Library    []LibraryItem `json:"library"`
	Plays      []Play        `json:"plays"`
}

type Repository interface {
	Export(context.Context, string) (Data, error)
}
type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}
func (service *Service) Data(ctx context.Context, userID string) (Data, error) {
	if userID == "" {
		return Data{}, fmt.Errorf("user is required")
	}
	data, err := service.repository.Export(ctx, userID)
	if err != nil {
		return Data{}, err
	}
	data.ExportedAt = service.now().UTC()
	return data, nil
}
