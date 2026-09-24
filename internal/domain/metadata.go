package domain

import "context"

// MetadataProvider is the domain boundary for external media metadata.
// Implementations may cache and rate-limit provider requests, but callers never
// depend on an external provider's client or response types.
type MetadataProvider interface {
	Search(ctx context.Context, query, language string) ([]MediaSearchResult, error)
}

type TrendingMetadataProvider interface {
	Trending(ctx context.Context, mediaType, timeWindow string) ([]MediaSearchResult, error)
}

type RelatedMetadataProvider interface {
	Related(ctx context.Context, mediaType MediaType, tmdbID int64) ([]MediaSearchResult, error)
}

type TVShowMetadataProvider interface {
	Show(ctx context.Context, tmdbID int64) (TVShowMetadata, error)
}

// ScheduledTVShowMetadataProvider returns a bounded refresh snapshot for the
// background catalog job. Implementations should avoid fetching every season.
type ScheduledTVShowMetadataProvider interface {
	RefreshShow(ctx context.Context, tmdbID int64) (TVShowMetadata, error)
}

// TVShowSummaryProvider provides the show-level details without loading every
// season's episodes. The HTTP layer uses this for temporary Discover views.
type TVShowSummaryProvider interface {
	ShowSummary(ctx context.Context, tmdbID int64) (TVShowMetadata, error)
	Season(ctx context.Context, tmdbID int64, seasonNumber int) (TVSeasonMetadata, error)
}

type MovieMetadataProvider interface {
	Movie(ctx context.Context, tmdbID int64) (MovieMetadata, error)
}

type TVEpisodeMetadataProvider interface {
	Episode(ctx context.Context, showID int64, seasonNumber int, episodeNumber int) (TVEpisodeMetadata, error)
}

type PersonMetadataProvider interface {
	Person(ctx context.Context, tmdbID int64) (PersonMetadata, error)
}

type PersonMetadata struct {
	TMDBID       int64          `json:"tmdb_id"`
	Name         string         `json:"name"`
	Biography    string         `json:"biography"`
	ProfilePath  string         `json:"profile_path,omitempty"`
	Birthday     string         `json:"birthday,omitempty"`
	Deathday     string         `json:"deathday,omitempty"`
	PlaceOfBirth string         `json:"place_of_birth,omitempty"`
	KnownFor     string         `json:"known_for_department,omitempty"`
	Credits      []PersonCredit `json:"credits"`
}

type PersonCredit struct {
	TMDBID           int64     `json:"tmdb_id"`
	Type             MediaType `json:"type"`
	Title            string    `json:"title"`
	OriginalTitle    string    `json:"original_title,omitempty"`
	Character        string    `json:"character,omitempty"`
	Overview         string    `json:"overview,omitempty"`
	ReleaseDate      string    `json:"release_date,omitempty"`
	PosterPath       string    `json:"poster_path,omitempty"`
	BackdropPath     string    `json:"backdrop_path,omitempty"`
	OriginalLanguage string    `json:"original_language,omitempty"`
	Popularity       float32   `json:"popularity"`
}

type TVShowMetadata struct {
	TMDBID           int64
	Name             string
	Overview         string
	PosterPath       string
	BackdropPath     string
	FirstAirDate     string
	OriginalLanguage string
	Status           string
	Seasons          []TVSeasonMetadata
	Cast             []TVCastMember
}

type TVCastMember struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path,omitempty"`
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
	TMDBID         int64
	SeasonNumber   int
	EpisodeNumber  int
	Name           string
	Overview       string
	AirDate        string
	Runtime        int
	StillPath      string
	VoteAverage    float32
	ProductionCode string
	GuestStars     []TVCastMember
	Crew           []TVCrewMember
}

type TVCrewMember struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path,omitempty"`
}

type MovieMetadata struct {
	TMDBID           int64
	Title            string
	OriginalTitle    string
	Overview         string
	PosterPath       string
	BackdropPath     string
	ReleaseDate      string
	OriginalLanguage string
	Runtime          int
	Status           string
	VoteAverage      float32
	Genres           []string
	Cast             []TVCastMember
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
	BackdropPath     string    `json:"backdrop_path,omitempty"`
}
