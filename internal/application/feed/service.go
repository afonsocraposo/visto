package feed

import (
	"context"
	"fmt"
	"time"
)

type Item struct {
	ID            string    `json:"id"`
	DisplayName   string    `json:"display_name"`
	Kind          string    `json:"kind"`
	Title         string    `json:"title"`
	MediaType     string    `json:"media_type,omitempty"`
	TMDBID        int       `json:"tmdb_id,omitempty"`
	ArtworkPath   string    `json:"artwork_path,omitempty"`
	Rating        *int      `json:"rating,omitempty"`
	Count         int       `json:"count,omitempty"`
	SeasonNumber  *int      `json:"season_number,omitempty"`
	EpisodeNumber *int      `json:"episode_number,omitempty"`
	EpisodeName   string    `json:"episode_name,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type Page struct {
	Items      []Item  `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

type Repository interface {
	List(context.Context, string, int) (Page, error)
}
type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }
func (service *Service) List(ctx context.Context, cursor string, limit int) (Page, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		return Page{}, fmt.Errorf("limit must not exceed 100")
	}
	return service.repository.List(ctx, cursor, limit)
}
