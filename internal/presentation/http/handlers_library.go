package httpserver

import "encoding/json"
import "errors"
import "fmt"
import "net/http"
import "strconv"
import "github.com/afonsocosta/visto/internal/application/auth"
import "github.com/afonsocosta/visto/internal/application/library"
import "github.com/afonsocosta/visto/internal/domain"

func removeWatchlistItem(authService *auth.Service, service *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "library is not configured")
			return
		}
		var err error
		status := r.URL.Query().Get("status")
		if status != "" && status != "watchlist" && status != "watching" {
			writeError(w, http.StatusBadRequest, "invalid library status")
			return
		}
		if status == "watching" {
			err = service.RemoveUnplayedWatchingItem(r.Context(), user.ID, r.PathValue("mediaID"))
		} else {
			err = service.RemoveWatchlistItem(r.Context(), user.ID, r.PathValue("mediaID"))
		}
		if err != nil {
			if errors.Is(err, library.ErrMediaNotFound) {
				writeError(w, http.StatusNotFound, "title is not in the selected list")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func updateLibrary(authService *auth.Service, service *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "library is not configured")
			return
		}
		var request struct {
			Status domain.LibraryStatus `json:"status"`
			Rating *int                 `json:"rating"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		item, err := service.Save(r.Context(), user.ID, r.PathValue("mediaID"), request.Status, request.Rating)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func setLibraryNotifications(authService *auth.Service, service *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "library is not configured")
			return
		}
		var request struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := service.SetNotificationsEnabled(r.Context(), user.ID, r.PathValue("mediaID"), request.Enabled); err != nil {
			if errors.Is(err, library.ErrMediaNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type libraryRequest struct {
	Media  library.Media        `json:"media"`
	Status domain.LibraryStatus `json:"status"`
	Rating *int                 `json:"rating"`
}

func saveLibrary(authService *auth.Service, service *library.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "library is not configured")
			return
		}
		var request libraryRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		item, err := service.SaveMedia(r.Context(), user.ID, request.Media, request.Status, request.Rating)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if request.Media.Type == domain.TVMediaType {
			tvProvider, ok := provider.(domain.TVShowMetadataProvider)
			if !ok {
				writeError(w, http.StatusServiceUnavailable, "TV metadata import is not configured")
				return
			}
			if err := service.ImportShow(r.Context(), request.Media.TMDBID, tvProvider); err != nil {
				writeError(w, http.StatusBadGateway, "could not import TV show metadata")
				return
			}
		}
		writeJSON(w, http.StatusCreated, item)
	}
}

func listLibrary(authService *auth.Service, service *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "library is not configured")
			return
		}
		items, err := service.List(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "library is temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func mediaDetails(authService *auth.Service, service *library.Service, provider domain.MetadataProvider, mediaType domain.MediaType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "library is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB ID")
			return
		}
		entry, err := service.GetByTMDBID(r.Context(), user.ID, mediaType, tmdbID)
		if err != nil {
			if errors.Is(err, library.ErrMediaNotFound) {
				if mediaType == domain.TVMediaType {
					if tvProvider, ok := provider.(domain.TVShowSummaryProvider); ok {
						show, providerErr := tvProvider.ShowSummary(r.Context(), tmdbID)
						if providerErr == nil {
							writeJSON(w, http.StatusOK, library.Entry{Media: library.Media{ID: fmt.Sprintf("tv:%d", show.TMDBID), Type: domain.TVMediaType, TMDBID: show.TMDBID, Title: show.Name, OriginalTitle: show.Name, Overview: show.Overview, ReleaseDate: show.FirstAirDate, PosterPath: show.PosterPath, OriginalLanguage: show.OriginalLanguage, Status: show.Status}})
							return
						}
					}
				}
				if mediaType == domain.MovieMediaType {
					if movieProvider, ok := provider.(domain.MovieMetadataProvider); ok {
						movie, providerErr := movieProvider.Movie(r.Context(), tmdbID)
						if providerErr == nil {
							writeJSON(w, http.StatusOK, library.Entry{Media: library.Media{ID: fmt.Sprintf("movie:%d", movie.TMDBID), Type: domain.MovieMediaType, TMDBID: movie.TMDBID, Title: movie.Title, OriginalTitle: movie.OriginalTitle, Overview: movie.Overview, ReleaseDate: movie.ReleaseDate, PosterPath: movie.PosterPath, BackdropPath: movie.BackdropPath, OriginalLanguage: movie.OriginalLanguage, Status: movie.Status}})
							return
						}
					}
				}
				writeError(w, http.StatusNotFound, "media not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "media details are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, entry)
	}
}
