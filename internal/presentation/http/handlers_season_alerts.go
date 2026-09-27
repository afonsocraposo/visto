package httpserver

import (
	"errors"
	"net/http"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/seasonalerts"
)

func seasonAlert(authService *auth.Service, service *seasonalerts.Service, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		seasonID := r.PathValue("seasonID")
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "season alerts are unavailable")
			return
		}
		if action == "get" {
			state, err := service.State(r.Context(), user.ID, seasonID)
			if err != nil {
				writeSeasonAlertError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, state)
			return
		}
		var err error
		if action == "subscribe" {
			err = service.Subscribe(r.Context(), user.ID, seasonID)
		} else {
			err = service.Cancel(r.Context(), user.ID, seasonID)
		}
		if err != nil {
			writeSeasonAlertError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeSeasonAlertError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, seasonalerts.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, seasonalerts.ErrNotWatching), errors.Is(err, seasonalerts.ErrAlreadyReady):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "season alerts are temporarily unavailable")
	}
}
