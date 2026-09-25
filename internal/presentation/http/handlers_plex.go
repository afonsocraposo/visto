package httpserver

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/plexsync"
)

func plexWebhookStatus(authService *auth.Service, service *plexsync.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		status, err := service.Status(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Plex sync status is temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func issuePlexWebhook(authService *auth.Service, service *plexsync.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		webhookURL, err := service.Issue(r.Context(), user.ID)
		if err != nil {
			if strings.Contains(err.Error(), "VISTO_PUBLIC_URL") {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "Could not create a Plex webhook URL")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"webhook_url": webhookURL})
	}
}

func revokePlexWebhook(authService *auth.Service, service *plexsync.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if err := service.Revoke(r.Context(), user.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not revoke the Plex webhook")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func plexWebhook(service *plexsync.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := r.PathValue("secret")
		if len(secret) < 40 || len(secret) > 80 {
			http.NotFound(w, r)
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			writeError(w, http.StatusBadRequest, "Plex webhook must use multipart form data")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseMultipartForm(64 << 10); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeError(w, http.StatusRequestEntityTooLarge, "Plex webhook payload is too large")
				return
			}
			writeError(w, http.StatusBadRequest, "Could not read Plex webhook form")
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		var payload string
		if r.MultipartForm != nil && len(r.MultipartForm.Value["payload"]) == 1 && len(r.MultipartForm.File) == 0 {
			payload = r.MultipartForm.Value["payload"][0]
		}
		if payload == "" || len(payload) > 1<<20 {
			writeError(w, http.StatusBadRequest, "Plex webhook payload is missing or too large")
			return
		}
		if err := service.Handle(r.Context(), secret, payload); err != nil {
			if errors.Is(err, plexsync.ErrWebhookNotFound) {
				http.NotFound(w, r)
				return
			}
			writeError(w, http.StatusInternalServerError, "Plex event could not be synchronized")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
