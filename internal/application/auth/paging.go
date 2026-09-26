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

func (service *Service) UsersPage(ctx context.Context, request pagination.Request) (pagination.Page[domain.User], error) {
	repo, ok := service.repository.(usersPageRepository)
	if !ok {
		return pagination.Page[domain.User]{}, fmt.Errorf("user paging is not configured")
	}
	return repo.ListUsersPage(ctx, request)
}
