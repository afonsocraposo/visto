package domain

import "context"

// MetadataProvider is the domain boundary for external media metadata.
// Implementations may cache and rate-limit provider requests, but callers never
// depend on an external provider's client or response types.
type MetadataProvider interface {
	Search(ctx context.Context, query, language string) ([]MediaSearchResult, error)
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
