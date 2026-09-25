package watch

import (
	"context"
	"fmt"
	"time"
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
			entries = append(entries, CalendarEntry{ShowID: show.ID, Title: show.Title, Episode: episode})
		}
	}
	return entries, nil
}
