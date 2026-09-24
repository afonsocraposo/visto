package domain

import "time"

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
	ID            string
	ShowID        string
	SeasonNumber  int
	EpisodeNumber int
	AirDate       *time.Time
}

func (episode Episode) IsRegular() bool {
	return episode.SeasonNumber > 0
}

func (episode Episode) IsReleasedAt(now time.Time) bool {
	return episode.AirDate == nil || !episode.AirDate.After(now)
}

// EpisodePlay records a single watch. Rewatches are represented by additional
// plays instead of a mutable counter.
type EpisodePlay struct {
	ID        string
	UserID    string
	EpisodeID string
	WatchedAt time.Time
}
