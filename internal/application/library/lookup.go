package library

import (
	"context"
	"fmt"
)

type lookupRepository interface {
	LookupItems(context.Context, string, []string) ([]Entry, error)
}

func (s *Service) Lookup(ctx context.Context, userID string, ids []string) ([]Entry, error) {
	if userID == "" || len(ids) > 100 {
		return nil, fmt.Errorf("invalid library lookup")
	}
	for _, id := range ids {
		if id == "" {
			return nil, fmt.Errorf("invalid library lookup")
		}
	}
	repo, ok := s.repository.(lookupRepository)
	if !ok {
		return nil, fmt.Errorf("library lookup is not configured")
	}
	if len(ids) == 0 {
		return []Entry{}, nil
	}
	return repo.LookupItems(ctx, userID, ids)
}
