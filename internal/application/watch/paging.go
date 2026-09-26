package watch

import (
	"context"
	"fmt"
	"github.com/afonsocosta/visto/internal/application/pagination"
)

type episodePageRepository interface {
	ListShowEpisodesPage(context.Context, string, string, pagination.Request) (pagination.Page[ShowEpisode], error)
	ListSeasonEpisodesPage(context.Context, string, string, pagination.Request) (pagination.Page[ShowEpisode], error)
}

func (service *Service) EpisodesPage(ctx context.Context, userID, showID string, request pagination.Request) (pagination.Page[ShowEpisode], error) {
	if userID == "" || showID == "" {
		return pagination.Page[ShowEpisode]{}, fmt.Errorf("user and show are required")
	}
	repo, ok := service.repository.(episodePageRepository)
	if !ok {
		return pagination.Page[ShowEpisode]{}, fmt.Errorf("episode paging is not configured")
	}
	return repo.ListShowEpisodesPage(ctx, userID, showID, request)
}
func (service *Service) SeasonEpisodesPage(ctx context.Context, userID, seasonID string, request pagination.Request) (pagination.Page[ShowEpisode], error) {
	if userID == "" || seasonID == "" {
		return pagination.Page[ShowEpisode]{}, fmt.Errorf("user and season are required")
	}
	repo, ok := service.repository.(episodePageRepository)
	if !ok {
		return pagination.Page[ShowEpisode]{}, fmt.Errorf("episode paging is not configured")
	}
	return repo.ListSeasonEpisodesPage(ctx, userID, seasonID, request)
}
