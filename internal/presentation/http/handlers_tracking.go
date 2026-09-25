package httpserver

import "encoding/json"
import "errors"
import "net/http"
import "strconv"
import "time"
import "github.com/afonsocosta/visto/internal/application/auth"
import "github.com/afonsocosta/visto/internal/application/tracking"

func playHistory(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid limit")
				return
			}
			limit = value
		}
		entries, err := service.History(r.Context(), user.ID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

func episodeRating(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		rating, err := service.EpisodeRating(r.Context(), user.ID, r.PathValue("episodeID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rating)
	}
}

func setEpisodeRating(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		var request struct {
			Rating *int `json:"rating"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		rating, err := service.RateEpisode(r.Context(), user.ID, r.PathValue("episodeID"), request.Rating)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rating)
	}
}

func createBulkPlays(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		var request struct {
			EpisodeIDs []string   `json:"episode_ids"`
			WatchedAt  *time.Time `json:"watched_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		watchedAt := time.Time{}
		if request.WatchedAt != nil {
			watchedAt = *request.WatchedAt
		}
		plays, err := service.RecordEpisodes(r.Context(), user.ID, request.EpisodeIDs, watchedAt, "web")
		if err != nil {
			writeTrackingError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, plays)
	}
}

func deleteBulkEpisodePlays(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		var request struct {
			EpisodeIDs []string `json:"episode_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := service.RemoveEpisodes(r.Context(), user.ID, request.EpisodeIDs); err != nil {
			writeTrackingError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteMediaPlays(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		if err := service.RemoveMediaPlays(r.Context(), user.ID, r.PathValue("mediaID")); err != nil {
			writeTrackingError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type playRequest struct {
	MediaID   *string    `json:"media_id"`
	EpisodeID *string    `json:"episode_id"`
	WatchedAt *time.Time `json:"watched_at"`
	Source    string     `json:"source"`
}

func createPlay(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		var request playRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		watchedAt := time.Time{}
		if request.WatchedAt != nil {
			watchedAt = *request.WatchedAt
		}
		play, err := service.Record(r.Context(), user.ID, request.MediaID, request.EpisodeID, watchedAt, request.Source)
		if err != nil {
			writeTrackingError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, play)
	}
}

func correctPlay(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		var request struct {
			WatchedAt time.Time `json:"watched_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := service.Correct(r.Context(), user.ID, r.PathValue("playID"), request.WatchedAt); err != nil {
			writeTrackingError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deletePlay(authService *auth.Service, service *tracking.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "tracking is not configured")
			return
		}
		if err := service.Remove(r.Context(), user.ID, r.PathValue("playID")); err != nil {
			writeTrackingError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeTrackingError(w http.ResponseWriter, err error) {
	if errors.Is(err, tracking.ErrPlayNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}
