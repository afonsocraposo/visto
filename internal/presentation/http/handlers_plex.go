package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/plexsync"
	"github.com/afonsocosta/visto/internal/domain"
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

func plexWebhookEventPayload(authService *auth.Service, service *plexsync.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "Invalid Plex event ID")
			return
		}
		payload, err := service.EventPayload(r.Context(), user.ID, id)
		if errors.Is(err, plexsync.ErrEventNotFound) {
			writeError(w, http.StatusNotFound, "Plex event not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Plex event payload is temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"raw_payload": payload})
	}
}

func issuePlexWebhook(authService *auth.Service, service *plexsync.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		var request struct {
			AccountID string `json:"account_id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid Plex webhook settings")
			return
		}
		webhookURL, err := service.Issue(r.Context(), user.ID, request.AccountID)
		if err != nil {
			if errors.Is(err, plexsync.ErrPersonalDisabled) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			if strings.Contains(err.Error(), "Plex account ID") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
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
			if errors.Is(err, plexsync.ErrPersonalDisabled) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "Could not revoke the Plex webhook")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func adminPlexSync(authService *auth.Service, service *plexsync.Service, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if actor.Role != domain.AdminRole {
			writeError(w, http.StatusForbidden, "administrator access required")
			return
		}
		switch action {
		case "get":
			status, err := service.AdminStatus(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Could not load Plex sync settings")
				return
			}
			writeJSON(w, http.StatusOK, status)
		case "issue":
			webhookURL, err := service.IssueShared(r.Context())
			if err != nil {
				if strings.Contains(err.Error(), "VISTO_PUBLIC_URL") {
					writeError(w, http.StatusServiceUnavailable, err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, "Could not create shared Plex webhook URL")
				return
			}
			writeJSON(w, http.StatusCreated, map[string]string{"webhook_url": webhookURL})
		case "revoke":
			if err := service.RevokeShared(r.Context()); err != nil {
				writeError(w, http.StatusInternalServerError, "Could not revoke shared Plex webhook URL")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "mapping":
			var body struct {
				AccountID string `json:"account_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid Plex account mapping")
				return
			}
			if err := service.SetMapping(r.Context(), r.PathValue("userID"), body.AccountID); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "unmap":
			if err := service.DeleteMapping(r.Context(), r.PathValue("userID")); err != nil {
				writeError(w, http.StatusInternalServerError, "Could not remove Plex account mapping")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
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
		if r.MultipartForm != nil {
			values := r.MultipartForm.Value["payload"]
			files := r.MultipartForm.File["payload"]
			if len(values)+len(files) == 1 {
				if len(values) == 1 {
					payload = values[0]
				} else {
					file, err := files[0].Open()
					if err != nil {
						writeError(w, http.StatusBadRequest, "Could not read Plex webhook form")
						return
					}
					data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
					file.Close()
					if err != nil {
						writeError(w, http.StatusBadRequest, "Could not read Plex webhook form")
						return
					}
					payload = string(data)
				}
			}
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
