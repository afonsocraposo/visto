package httpserver

import "fmt"
import "net/http"
import "strconv"
import "time"
import "github.com/afonsocosta/visto/internal/application/auth"
import "github.com/afonsocosta/visto/internal/application/library"
import "github.com/afonsocosta/visto/internal/application/watch"
import "github.com/afonsocosta/visto/internal/domain"

func trending(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		serveTrending(w, r, provider)
	}
}

func publicTrending(provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveTrending(w, r, provider)
	}
}

func relatedMedia(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		relatedProvider, ok := provider.(domain.RelatedMetadataProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "related media metadata is not configured")
			return
		}
		mediaType := domain.MediaType(r.PathValue("mediaType"))
		if mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType {
			writeError(w, http.StatusBadRequest, "media type must be movie or tv")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB ID")
			return
		}
		results, err := relatedProvider.Related(r.Context(), mediaType, tmdbID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "related titles are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, results)
	}
}

func serveTrending(w http.ResponseWriter, r *http.Request, provider domain.MetadataProvider) {
	type response struct {
		TV     []domain.MediaSearchResult `json:"tv"`
		Movies []domain.MediaSearchResult `json:"movies"`
	}
	trendingProvider, ok := provider.(domain.TrendingMetadataProvider)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "TMDB trending is not configured")
		return
	}
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "week"
	}
	if window != "day" && window != "week" {
		writeError(w, http.StatusBadRequest, "window must be day or week")
		return
	}
	movies, err := trendingProvider.Trending(r.Context(), "movie", window)
	if err != nil {
		writeError(w, http.StatusBadGateway, "trending metadata is temporarily unavailable")
		return
	}
	shows, err := trendingProvider.Trending(r.Context(), "tv", window)
	if err != nil {
		writeError(w, http.StatusBadGateway, "trending metadata is temporarily unavailable")
		return
	}
	writeJSON(w, http.StatusOK, response{TV: shows, Movies: movies})
}

type temporaryShowDetailsResponse struct {
	Media   library.Media         `json:"media"`
	Seasons []temporaryShowSeason `json:"seasons"`
	Cast    []domain.TVCastMember `json:"cast"`
}

type temporaryShowSeason struct {
	TMDBID     int64               `json:"tmdb_id"`
	Number     int                 `json:"season_number"`
	Name       string              `json:"name"`
	Overview   string              `json:"overview,omitempty"`
	PosterPath string              `json:"poster_path,omitempty"`
	AirDate    string              `json:"air_date,omitempty"`
	Episodes   []watch.ShowEpisode `json:"episodes"`
}

type temporaryMovieDetailsResponse struct {
	Media       library.Media         `json:"media"`
	Runtime     int                   `json:"runtime,omitempty"`
	VoteAverage float32               `json:"vote_average,omitempty"`
	Genres      []string              `json:"genres,omitempty"`
	Cast        []domain.TVCastMember `json:"cast"`
}

type temporaryEpisodeDetailsResponse struct {
	Name           string                `json:"name"`
	Overview       string                `json:"overview,omitempty"`
	AirDate        string                `json:"air_date,omitempty"`
	Runtime        int                   `json:"runtime,omitempty"`
	StillPath      string                `json:"still_path,omitempty"`
	VoteAverage    float32               `json:"vote_average,omitempty"`
	ProductionCode string                `json:"production_code,omitempty"`
	GuestStars     []domain.TVCastMember `json:"guest_stars,omitempty"`
	Crew           []domain.TVCrewMember `json:"crew,omitempty"`
}

func temporaryShowDetails(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		tvProvider, ok := provider.(domain.TVShowSummaryProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "TV summary metadata is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB ID")
			return
		}
		show, err := tvProvider.ShowSummary(r.Context(), tmdbID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "TV details are temporarily unavailable")
			return
		}
		response := temporaryShowDetailsResponse{
			Media: library.Media{ID: fmt.Sprintf("tv:%d", show.TMDBID), Type: domain.TVMediaType, TMDBID: show.TMDBID, Title: show.Name, OriginalTitle: show.Name, Overview: show.Overview, ReleaseDate: show.FirstAirDate, PosterPath: show.PosterPath, BackdropPath: show.BackdropPath, OriginalLanguage: show.OriginalLanguage, Status: show.Status},
			Cast:  show.Cast,
		}
		for _, season := range show.Seasons {
			response.Seasons = append(response.Seasons, temporaryShowSeason{TMDBID: season.TMDBID, Number: season.Number, Name: season.Name, Overview: season.Overview, PosterPath: season.PosterPath, AirDate: season.AirDate, Episodes: nil})
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func temporaryShowSeasonEpisodes(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		tvProvider, ok := provider.(domain.TVShowSummaryProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "TV season metadata is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		seasonNumber, seasonErr := strconv.Atoi(r.PathValue("seasonNumber"))
		if err != nil || tmdbID <= 0 || seasonErr != nil || seasonNumber < 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB show or season")
			return
		}
		season, err := tvProvider.Season(r.Context(), tmdbID, seasonNumber)
		if err != nil {
			writeError(w, http.StatusBadGateway, "TV season details are temporarily unavailable")
			return
		}
		response := temporaryShowSeason{TMDBID: season.TMDBID, Number: season.Number, Name: season.Name, Overview: season.Overview, PosterPath: season.PosterPath, AirDate: season.AirDate, Episodes: []watch.ShowEpisode{}}
		for _, episode := range season.Episodes {
			var airDate *time.Time
			if parsed, parseErr := time.Parse(time.DateOnly, episode.AirDate); parseErr == nil {
				airDate = &parsed
			}
			response.Episodes = append(response.Episodes, watch.ShowEpisode{Episode: domain.Episode{ID: fmt.Sprintf("tv:%d:episode:%d", tmdbID, episode.TMDBID), ShowID: fmt.Sprintf("tv:%d", tmdbID), SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber, AirDate: airDate}, Name: episode.Name, Overview: episode.Overview, Runtime: episode.Runtime, StillPath: episode.StillPath})
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func temporaryMovieDetails(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		movieProvider, ok := provider.(domain.MovieMetadataProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "movie metadata is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB ID")
			return
		}
		movie, err := movieProvider.Movie(r.Context(), tmdbID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "movie details are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, temporaryMovieDetailsResponse{
			Media:   library.Media{ID: fmt.Sprintf("movie:%d", movie.TMDBID), Type: domain.MovieMediaType, TMDBID: movie.TMDBID, Title: movie.Title, OriginalTitle: movie.OriginalTitle, Overview: movie.Overview, ReleaseDate: movie.ReleaseDate, PosterPath: movie.PosterPath, BackdropPath: movie.BackdropPath, OriginalLanguage: movie.OriginalLanguage, Status: movie.Status},
			Runtime: movie.Runtime, VoteAverage: movie.VoteAverage, Genres: movie.Genres, Cast: movie.Cast,
		})
	}
}

func personDetails(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		personProvider, ok := provider.(domain.PersonMetadataProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "person metadata is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB person ID")
			return
		}
		person, err := personProvider.Person(r.Context(), tmdbID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "person details are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, person)
	}
}

func temporaryEpisodeDetails(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		episodeProvider, ok := provider.(domain.TVEpisodeMetadataProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "episode metadata is not configured")
			return
		}
		showID, showErr := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		seasonNumber, seasonErr := strconv.Atoi(r.PathValue("seasonNumber"))
		episodeNumber, episodeErr := strconv.Atoi(r.PathValue("episodeNumber"))
		if showErr != nil || showID <= 0 || seasonErr != nil || seasonNumber < 0 || episodeErr != nil || episodeNumber <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TV episode")
			return
		}
		episode, err := episodeProvider.Episode(r.Context(), showID, seasonNumber, episodeNumber)
		if err != nil {
			writeError(w, http.StatusBadGateway, "episode details are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, temporaryEpisodeDetailsResponse{Name: episode.Name, Overview: episode.Overview, AirDate: episode.AirDate, Runtime: episode.Runtime, StillPath: episode.StillPath, VoteAverage: episode.VoteAverage, ProductionCode: episode.ProductionCode, GuestStars: episode.GuestStars, Crew: episode.Crew})
	}
}

func search(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		if provider == nil {
			writeError(w, http.StatusServiceUnavailable, "metadata search is not configured")
			return
		}
		query := r.URL.Query().Get("q")
		if query == "" {
			writeError(w, http.StatusBadRequest, "q is required")
			return
		}
		results, err := provider.Search(r.Context(), query, r.URL.Query().Get("language"))
		if err != nil {
			writeError(w, http.StatusBadGateway, "metadata search is temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, results)
	}
}
