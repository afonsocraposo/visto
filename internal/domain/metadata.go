package domain

import "context"

// MetadataProvider is the domain boundary for external media metadata.
// Implementations may cache and rate-limit provider requests, but callers never
// depend on an external provider's client or response types.
type MetadataProvider interface {
	Search(ctx context.Context, query, language string) ([]MediaSearchResult, error)
}

type MediaSearchResult struct {
	TMDBID           int64
	Type             MediaType
	Title            string
	OriginalTitle    string
	Overview         string
	ReleaseDate      string
	PosterPath       string
	OriginalLanguage string
}
