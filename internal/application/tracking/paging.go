package tracking

import (
	"context"
	"fmt"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"strings"
)

type pageRepository interface {
	ListPlaysPage(context.Context, string, string, string, pagination.Request) (pagination.Page[HistoryEntry], error)
}

func (service *Service) HistoryPage(ctx context.Context, userID, mediaID, episodeID string, request pagination.Request) (pagination.Page[HistoryEntry], error) {
	if userID == "" || (mediaID != "" && !strings.HasPrefix(mediaID, "movie:")) || (mediaID != "" && episodeID != "") {
		return pagination.Page[HistoryEntry]{}, fmt.Errorf("invalid history request")
	}
	repo, ok := service.repository.(pageRepository)
	if !ok {
		return pagination.Page[HistoryEntry]{}, fmt.Errorf("history paging is not configured")
	}
	return repo.ListPlaysPage(ctx, userID, mediaID, episodeID, request)
}
