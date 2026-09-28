package httpserver

import (
	"errors"
	"io"
	"net/http"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/importer"
)

func importWelcome(authService *auth.Service, service *importer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		pending, err := service.WelcomePending(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not read import welcome")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"pending": pending})
	}
}

func dismissImportWelcome(authService *auth.Service, service *importer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if err := service.DismissWelcome(r.Context(), user.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not dismiss import welcome")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func importBingers(authService *auth.Service, service *importer.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if r.Header.Get("Content-Type") != "application/zip" {
			writeError(w, http.StatusUnsupportedMediaType, "upload a Bingers ZIP archive")
			return
		}
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "archive is too large")
			return
		}
		result, err := service.Import(r.Context(), user.ID, "bingers", payload)
		if err != nil {
			if errors.Is(err, importer.ErrInvalidArchive) {
				writeError(w, http.StatusBadRequest, err.Error())
			} else {
				writeError(w, http.StatusInternalServerError, "could not import archive")
			}
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}
