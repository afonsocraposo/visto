package domain

import (
	"encoding/json"
	"time"
)

type MediaType string

const (
	MovieMediaType MediaType = "movie"
	TVMediaType    MediaType = "tv"
)

type LibraryStatus string

const (
	WatchlistStatus LibraryStatus = "watchlist"
	WatchingStatus  LibraryStatus = "watching"
	PausedStatus    LibraryStatus = "paused"
	DroppedStatus   LibraryStatus = "dropped"
)

// Episode is a locally cached TMDB episode. Its ID, rather than its displayed
// season and episode numbers, is its permanent Visto identity.
type Episode struct {
	ID            string     `json:"id"`
	ShowID        string     `json:"show_id"`
	SeasonNumber  int        `json:"season_number"`
	EpisodeNumber int        `json:"episode_number"`
	AirDate       *time.Time `json:"air_date"`
}

func (episode Episode) IsRegular() bool {
	return episode.SeasonNumber > 0
}

func (episode Episode) IsReleasedAt(now time.Time) bool {
	return episode.AirDate == nil || episode.AirDate.UTC().Format(time.DateOnly) <= now.In(now.Location()).Format(time.DateOnly)
}

func (episode Episode) MarshalJSON() ([]byte,error) {
	type response struct {
		ID string `json:"id"`
		ShowID string `json:"show_id"`
		SeasonNumber int `json:"season_number"`
		EpisodeNumber int `json:"episode_number"`
		AirDate *string `json:"air_date"`
	}
	result:=response{ID:episode.ID,ShowID:episode.ShowID,SeasonNumber:episode.SeasonNumber,EpisodeNumber:episode.EpisodeNumber}
	if episode.AirDate!=nil{value:=episode.AirDate.UTC().Format(time.DateOnly);result.AirDate=&value}
	return json.Marshal(result)
}

// EpisodePlay records a single watch. Rewatches are represented by additional
// plays instead of a mutable counter.
type EpisodePlay struct {
	ID        string
	UserID    string
	EpisodeID string
	WatchedAt time.Time
}
