package library

import (
	"context"
	"fmt"
	"github.com/afonsocosta/visto/internal/application/pagination"
)

type ListOptions struct {
	Sort      string
	Status    string
	MediaType string
}
type pageRepository interface {
	ListItemsPage(context.Context, string, ListOptions, pagination.Request) (pagination.Page[Entry], error)
}

func (s *Service) ListPage(ctx context.Context, userID string, options ListOptions, request pagination.Request) (pagination.Page[Entry], error) {
	if userID == "" {
		return pagination.Page[Entry]{}, fmt.Errorf("user is required")
	}
	if options.Sort == "" {
		options.Sort = "updated"
	}
	if options.Sort != "updated" && options.Sort != "title" && options.Sort != "released" {
		return pagination.Page[Entry]{}, fmt.Errorf("invalid library sort")
	}
	if options.Status != "" && options.Status != "watching" && options.Status != "watchlist" && options.Status != "paused" && options.Status != "dropped" && options.Status != "completed" {
		return pagination.Page[Entry]{}, fmt.Errorf("invalid library status")
	}
	if options.MediaType != "" && options.MediaType != "movie" && options.MediaType != "tv" {
		return pagination.Page[Entry]{}, fmt.Errorf("invalid media type")
	}
	repo, ok := s.repository.(pageRepository)
	if !ok {
		return pagination.Page[Entry]{}, fmt.Errorf("library paging is not configured")
	}
	return repo.ListItemsPage(ctx, userID, options, request)
}
