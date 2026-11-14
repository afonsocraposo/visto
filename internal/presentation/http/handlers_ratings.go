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
