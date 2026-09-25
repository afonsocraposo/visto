package sqlite

import (
	"context"

	"github.com/afonsocosta/visto/internal/application/tracking"
)

var _ tracking.Repository = (*Store)(nil)

var _ interface {
	ListPlays(context.Context, string, int) ([]tracking.HistoryEntry, error)
} = (*Store)(nil)
