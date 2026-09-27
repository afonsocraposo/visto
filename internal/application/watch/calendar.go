package watch

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/afonsocosta/visto/internal/application/pagination"
)

func (service *Service) Calendar(ctx context.Context, userID string, from, to time.Time) ([]CalendarEntry, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	fromProvided, toProvided := !from.IsZero(), !to.IsZero()
	zoneName, err := service.repository.Timezone(ctx, userID)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(zoneName)
	if err != nil {
		return nil, fmt.Errorf("invalid user timezone")
	}
	fromDate := service.now().In(location).Format(time.DateOnly)
	if fromProvided {
		fromDate = from.UTC().Format(time.DateOnly)
	}
	fromDateValue, err := time.Parse(time.DateOnly, fromDate)
	if err != nil {
		return nil, fmt.Errorf("invalid calendar start date")
	}
	toDate := fromDateValue.AddDate(0, 0, 30).Format(time.DateOnly)
	if toProvided {
		toDate = to.UTC().Format(time.DateOnly)
	}
	toDateValue, err := time.Parse(time.DateOnly, toDate)
	if err != nil || toDateValue.Before(fromDateValue) || toDateValue.Sub(fromDateValue) > 366*24*time.Hour {
		return nil, fmt.Errorf("calendar range must be ordered and no longer than one year")
	}
	shows, err := service.repository.WatchingShows(ctx, userID)
	if err != nil {
		return nil, err
	}
	played := map[string]map[string]bool{}
	for _, show := range shows {
		played[show.ID] = map[string]bool{}
		for _, play := range show.Plays {
			played[show.ID][play.EpisodeID] = true
		}
	}
	entries := []CalendarEntry{}
	for _, show := range shows {
		for _, episode := range show.Episodes {
			if !episode.IsRegular() || episode.AirDate == nil || played[show.ID][episode.ID] || episode.AirDate.UTC().Format("2006-01-02") < fromDate || episode.AirDate.UTC().Format("2006-01-02") > toDate {
				continue
			}
			details := show.EpisodeDetails[episode.ID]
			entries = append(entries, CalendarEntry{
				ShowID: show.ID, Title: show.Title, PosterPath: show.PosterPath,
				Episode: episode, EpisodeName: details.Name, EpisodeStillPath: details.StillPath,
			})
		}
	}
	return entries, nil
}

func (service *Service) CalendarPage(ctx context.Context, userID string, from, to time.Time, request pagination.Request) (pagination.Page[CalendarEntry], error) {
	entries, err := service.Calendar(ctx, userID, from, to)
	if err != nil {
		return pagination.Page[CalendarEntry]{}, err
	}
	scope := "calendar|" + userID + "|" + from.Format(time.DateOnly) + "|" + to.Format(time.DateOnly)
	keys, err := pagination.Decode(request.Cursor, scope, 5)
	if err != nil {
		return pagination.Page[CalendarEntry]{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return calendarKeyCompare(calendarEntryKey(entries[i]), calendarEntryKey(entries[j])) < 0
	})
	start := 0
	if keys != nil {
		found := false
		for i, entry := range entries {
			if calendarKeyCompare(calendarEntryKey(entry), keys) == 0 {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return pagination.Page[CalendarEntry]{}, pagination.ErrInvalidCursor
		}
	}
	return pagination.Slice(entries[start:], request.Limit, func(entry CalendarEntry) string {
		return pagination.Encode(scope, calendarEntryKey(entry)...)
	}), nil
}

func calendarEntryKey(entry CalendarEntry) []string {
	return []string{
		entry.Episode.AirDate.UTC().Format(time.DateOnly),
		entry.Title,
		fmt.Sprintf("%08d", entry.Episode.SeasonNumber),
		fmt.Sprintf("%08d", entry.Episode.EpisodeNumber),
		entry.Episode.ID,
	}
}

func calendarKeyCompare(left, right []string) int {
	for i := range left {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	return 0
}

func (service *Service) CalendarDates(ctx context.Context, userID string, from, to time.Time) ([]string, error) {
	entries, err := service.Calendar(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	dates := make(map[string]struct{})
	for _, entry := range entries {
		dates[entry.Episode.AirDate.UTC().Format(time.DateOnly)] = struct{}{}
	}
	result := make([]string, 0, len(dates))
	for date := range dates {
		result = append(result, date)
	}
	sort.Strings(result)
	return result, nil
}
