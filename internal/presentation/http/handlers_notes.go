package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/library"
)

func titleNote(authService *auth.Service, service *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "library is not configured")
			return
		}
		mediaID := r.PathValue("mediaID")
		load, save := service.Note, service.SetNote
		if episodeID := r.PathValue("episodeID"); episodeID != "" {
			mediaID, load, save = episodeID, service.EpisodeNote, service.SetEpisodeNote
		}
		if r.Method == http.MethodPut {
			var request struct {
				Note *string `json:"note"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Note == nil {
				writeError(w, http.StatusBadRequest, "note is required; use an empty string to delete it")
				return
			}
			err := save(r.Context(), user.ID, mediaID, *request.Note)
			if errors.Is(err, library.ErrMediaNotFound) {
				writeError(w, http.StatusNotFound, "item not found")
				return
			}
			if err != nil {
				if errors.Is(err, library.ErrInvalidNote) {
					writeError(w, http.StatusBadRequest, err.Error())
				} else {
					writeError(w, http.StatusInternalServerError, "could not save your note")
				}
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"note": *request.Note})
			return
		}
		note, err := load(r.Context(), user.ID, mediaID)
		if errors.Is(err, library.ErrMediaNotFound) {
			writeError(w, http.StatusNotFound, "item not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load your note")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"note": note})
	}
}
