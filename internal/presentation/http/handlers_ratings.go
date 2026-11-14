package httpserver

import (
	"net/http"
	"strconv"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/ratings"
	"github.com/afonsocosta/visto/internal/domain"
)

func externalRatings(authService *auth.Service, service *ratings.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		kind := domain.MediaType(r.PathValue("mediaType"))
		if err != nil || id <= 0 || (kind != domain.MovieMediaType && kind != domain.TVMediaType) {
			writeError(w, http.StatusBadRequest, "valid media type and TMDB ID are required")
			return
		}
		result, err := service.Get(r.Context(), kind, id)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "ratings are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func externalEpisodeRatings(authService *auth.Service, service *ratings.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		id, idErr := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		season, seasonErr := strconv.Atoi(r.PathValue("seasonNumber"))
		episode, episodeErr := strconv.Atoi(r.PathValue("episodeNumber"))
		if idErr != nil || seasonErr != nil || episodeErr != nil || id <= 0 || season < 0 || episode <= 0 {
			writeError(w, http.StatusBadRequest, "valid show, season and episode numbers are required")
			return
		}
		result, err := service.GetEpisode(r.Context(), id, season, episode)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "episode ratings are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}
