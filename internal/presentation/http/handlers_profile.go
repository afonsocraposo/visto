package httpserver

import "encoding/json"
import "errors"
import "fmt"
import "net/http"
import "github.com/afonsocosta/visto/internal/application/auth"
import "github.com/afonsocosta/visto/internal/application/feed"
import "github.com/afonsocosta/visto/internal/application/profile"

func instanceFeed(authService *auth.Service, service *feed.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "feed is not configured")
			return
		}
		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil {
				writeError(w, http.StatusBadRequest, "invalid limit")
				return
			}
		}
		page, err := service.List(r.Context(), r.URL.Query().Get("cursor"), limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

func activitySettings(authService *auth.Service, service *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "profile is not configured")
			return
		}
		settings, err := service.Get(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "profile is temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func setActivitySettings(authService *auth.Service, service *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "profile is not configured")
			return
		}
		var request profile.Settings
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := service.Update(r.Context(), user.ID, request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func setPushoverSettings(authService *auth.Service, service *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "profile is not configured")
			return
		}
		var request struct {
			Enabled  bool   `json:"enabled"`
			AppToken string `json:"app_token"`
			UserKey  string `json:"user_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := service.UpdatePushover(r.Context(), user.ID, request.Enabled, request.AppToken, request.UserKey); err != nil {
			if errors.Is(err, profile.ErrPushoverUnavailable) {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			if errors.Is(err, profile.ErrInvalidPushoverSettings) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "Pushover settings could not be saved")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func clearPushoverCredentials(authService *auth.Service, service *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "profile is not configured")
			return
		}
		if err := service.ClearPushoverCredentials(r.Context(), user.ID); err != nil {
			if errors.Is(err, profile.ErrPushoverUnavailable) {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "Pushover credentials could not be removed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
