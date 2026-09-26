package sqlite

import (
	"context"
	"fmt"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"github.com/afonsocosta/visto/internal/domain"
	"strconv"
	"time"
)

func (store *Store) ListUsersPage(ctx context.Context, request pagination.Request) (pagination.Page[domain.User], error) {
	keys, err := pagination.Decode(request.Cursor, "users", 2)
	if err != nil {
		return pagination.Page[domain.User]{}, err
	}
	where := ""
	args := []any{}
	if keys != nil {
		if _, err := time.Parse(time.RFC3339Nano, keys[0]); err != nil {
			return pagination.Page[domain.User]{}, fmt.Errorf("invalid cursor")
		}
		if _, err := strconv.ParseInt(keys[1], 10, 64); err != nil {
			return pagination.Page[domain.User]{}, fmt.Errorf("invalid cursor")
		}
		where = "WHERE created_at>? OR (created_at=? AND id>?)"
		args = append(args, keys[0], keys[0], keys[1])
	}
	args = append(args, request.Limit+1)
	rows, err := store.DB.QueryContext(ctx, `SELECT id,email,display_name,'',role,created_at FROM users `+where+` ORDER BY created_at,id LIMIT ?`, args...)
	if err != nil {
		return pagination.Page[domain.User]{}, err
	}
	defer rows.Close()
	items := []domain.User{}
	for rows.Next() {
		user, _, err := scanUser(rows)
		if err != nil {
			return pagination.Page[domain.User]{}, err
		}
		items = append(items, user)
	}
	if err := rows.Err(); err != nil {
		return pagination.Page[domain.User]{}, err
	}
	return pagination.Slice(items, request.Limit, func(user domain.User) string {
		return pagination.Encode("users", user.CreatedAt.Format(time.RFC3339Nano), user.ID)
	}), nil
}
