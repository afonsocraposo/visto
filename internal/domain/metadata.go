package domain

import "context"

// MetadataProvider is the domain boundary for external media metadata.
// Implementations may cache and rate-limit provider requests, but callers never
// depend on an external provider's client or response types.
type MetadataProvider interface {
	Search(ctx context.Context, query, language string) ([]MediaSearchResult, error)
}

type TVShowMetadataProvider interface {
	Show(ctx context.Context, tmdbID int64) (TVShowMetadata, error)
}

type TVShowMetadata struct {
	TMDBID           int64
	Name             string
	Overview         string
	PosterPath       string
	FirstAirDate     string
	OriginalLanguage string
	Seasons          []TVSeasonMetadata
}

type TVSeasonMetadata struct {
	TMDBID     int64
	Number     int
	Name       string
	Overview   string
	PosterPath string
	AirDate    string
	Episodes   []TVEpisodeMetadata
}

type TVEpisodeMetadata struct {
	TMDBID        int64
	SeasonNumber  int
	EpisodeNumber int
	Name          string
	Overview      string
	AirDate       string
	Runtime       int
	StillPath     string
}

type MediaSearchResult struct {
	TMDBID           int64     `json:"tmdb_id"`
	Type             MediaType `json:"type"`
	Title            string    `json:"title"`
	OriginalTitle    string    `json:"original_title"`
	Overview         string    `json:"overview"`
	ReleaseDate      string    `json:"release_date"`
	PosterPath       string    `json:"poster_path"`
	OriginalLanguage string    `json:"original_language"`
}
