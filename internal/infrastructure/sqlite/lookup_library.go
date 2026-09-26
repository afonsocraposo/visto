package sqlite

import (
	"context"
	"github.com/afonsocosta/visto/internal/application/library"
)

func (s *Store) LookupItems(ctx context.Context, userID string, ids []string) ([]library.Entry, error) {
	return s.listItemsSorted(ctx, userID, "updated", ids)
}
