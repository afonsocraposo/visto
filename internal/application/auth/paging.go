package auth

import (
	"context"
	"fmt"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"github.com/afonsocosta/visto/internal/domain"
)

type usersPageRepository interface {
	ListUsersPage(context.Context, pagination.Request) (pagination.Page[domain.User], error)
}

type communityUserRepository interface {
	FindUserByID(context.Context, string) (domain.User, error)
}

func (service *Service) CommunityUser(ctx context.Context, userID string) (domain.User, error) {
	repo, ok := service.repository.(communityUserRepository)
	if !ok {
		return domain.User{}, fmt.Errorf("user lookup is not configured")
	}
	return repo.FindUserByID(ctx, userID)
}

func (service *Service) UsersPage(ctx context.Context, request pagination.Request) (pagination.Page[domain.User], error) {
	repo, ok := service.repository.(usersPageRepository)
	if !ok {
		return pagination.Page[domain.User]{}, fmt.Errorf("user paging is not configured")
	}
	return repo.ListUsersPage(ctx, request)
}
