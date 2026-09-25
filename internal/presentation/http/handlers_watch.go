package httpserver

import "errors"
import "net/http"
import "time"
import "github.com/afonsocosta/visto/internal/application/auth"
import "github.com/afonsocosta/visto/internal/application/watch"

func continueWatching(authService *auth.Service, service *watch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "watch data is not configured")
			return
		}
		entries, err := service.Continue(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "watch data is temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

func showEpisodes(authService *auth.Service, service *watch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "show data is not configured")
			return
		}
		entries, err := service.Episodes(r.Context(), user.ID, r.PathValue("showID"))
		if err != nil {
			if errors.Is(err, watch.ErrShowNotFound) {
				writeError(w, http.StatusNotFound, "show not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "show episodes are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

func showProgress(authService *auth.Service, service *watch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "show data is not configured")
			return
		}
		progress, err := service.ShowProgress(r.Context(), user.ID, r.PathValue("showID"))
		if err != nil {
			if errors.Is(err, watch.ErrShowNotFound) {
				writeError(w, http.StatusNotFound, "show not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "show progress is temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, progress)
	}
}

func showSeasons(authService *auth.Service, service *watch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "show data is not configured")
			return
		}
		seasons, err := service.Seasons(r.Context(), user.ID, r.PathValue("showID"))
		if err != nil {
			if errors.Is(err, watch.ErrShowNotFound) {
				writeError(w, http.StatusNotFound, "show not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "show seasons are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, seasons)
	}
}

func seasonEpisodes(authService *auth.Service, service *watch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "season data is not configured")
			return
		}
		entries, err := service.SeasonEpisodes(r.Context(), user.ID, r.PathValue("seasonID"))
		if err != nil {
			if errors.Is(err, watch.ErrShowNotFound) {
				writeError(w, http.StatusNotFound, "season not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "season episodes are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

func calendar(authService *auth.Service, service *watch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "watch data is not configured")
			return
		}
		from, to := time.Time{}, time.Time{}
		var err error
		if value := r.URL.Query().Get("from"); value != "" {
			from, err = time.Parse("2006-01-02", value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "from must use YYYY-MM-DD")
				return
			}
		}
		if value := r.URL.Query().Get("to"); value != "" {
			to, err = time.Parse("2006-01-02", value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "to must use YYYY-MM-DD")
				return
			}
		}
		entries, err := service.Calendar(r.Context(), user.ID, from, to)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}
