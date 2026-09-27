package watch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/pagination"
	"github.com/afonsocosta/visto/internal/domain"
)

func TestCalendarPageOrdersAcrossDateBoundaryAndRejectsWrongCursor(t *testing.T) {
	first := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 0, 1)
	service := NewService(repository{timezone: "UTC", shows: []Show{{
		ID: "tv:1", Title: "Show", Episodes: []domain.Episode{
			{ID: "c", SeasonNumber: 1, EpisodeNumber: 3, AirDate: &second},
			{ID: "b", SeasonNumber: 1, EpisodeNumber: 2, AirDate: &first},
			{ID: "a", SeasonNumber: 1, EpisodeNumber: 1, AirDate: &first},
		},
	}}})
	to := second.AddDate(0, 0, 1)
	page, err := service.CalendarPage(context.Background(), "user", first, to, pagination.Request{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.Items[0].Episode.ID != "a" || page.Items[1].Episode.ID != "b" || page.NextCursor == nil {
		t.Fatalf("first page = %+v, %v", page, err)
	}
	next, err := service.CalendarPage(context.Background(), "user", first, to, pagination.Request{Limit: 2, Cursor: *page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].Episode.ID != "c" || next.NextCursor != nil {
		t.Fatalf("next page = %+v, %v", next, err)
	}
	if _, err := service.CalendarPage(context.Background(), "other", first, to, pagination.Request{Limit: 2, Cursor: *page.NextCursor}); !errors.Is(err, pagination.ErrInvalidCursor) {
		t.Fatalf("wrong user cursor error = %v", err)
	}
	if _, err := service.CalendarPage(context.Background(), "user", first, to, pagination.Request{Limit: 2, Cursor: "invalid"}); !errors.Is(err, pagination.ErrInvalidCursor) {
		t.Fatalf("malformed cursor error = %v", err)
	}
	dates, err := service.CalendarDates(context.Background(), "user", first, to)
	if err != nil || len(dates) != 2 || dates[0] != "2026-10-01" || dates[1] != "2026-10-02" {
		t.Fatalf("dates = %v, %v", dates, err)
	}
}
