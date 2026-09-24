package httpserver

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	exportapp "github.com/afonsocosta/visto/internal/application/export"
	"github.com/afonsocosta/visto/internal/application/feed"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/profile"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
)

type Server struct {
	handler http.Handler
}

func New(authService *auth.Service, metadataProvider domain.MetadataProvider, webDir string, libraryService *library.Service, trackingService *tracking.Service, profileService *profile.Service, feedService *feed.Service, exportService *exportapp.Service, watchService *watch.Service) *Server {
	mux := http.NewServeMux()
	loginLimiter := newLoginLimiter()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", bootstrap(authService))
	mux.HandleFunc("GET /api/v1/auth/status", bootstrapStatus(authService))
	mux.HandleFunc("POST /api/v1/auth/login", login(authService, loginLimiter))
	mux.HandleFunc("POST /api/v1/auth/logout", logout(authService))
	mux.HandleFunc("POST /api/v1/users", createUser(authService))
	mux.HandleFunc("GET /api/v1/me", currentUser(authService))
	mux.HandleFunc("GET /api/v1/tokens", listPersonalTokens(authService))
	mux.HandleFunc("POST /api/v1/tokens", createPersonalToken(authService))
	mux.HandleFunc("DELETE /api/v1/tokens/{tokenID}", revokePersonalToken(authService))
	mux.HandleFunc("GET /api/v1/profile/activity-settings", activitySettings(authService, profileService))
	mux.HandleFunc("PATCH /api/v1/profile/activity-settings", setActivitySettings(authService, profileService))
	mux.HandleFunc("PATCH /api/v1/profile/pushover-settings", setPushoverSettings(authService, profileService))
	mux.HandleFunc("DELETE /api/v1/profile/pushover-key", clearPushoverKey(authService, profileService))
	mux.HandleFunc("GET /api/v1/feed", instanceFeed(authService, feedService))
	mux.HandleFunc("GET /api/v1/export/json", jsonExport(authService, exportService))
	mux.HandleFunc("GET /api/v1/export/csv", csvExport(authService, exportService))
	mux.HandleFunc("GET /api/v1/search", search(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/trending", trending(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/public/trending", publicTrending(metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/shows/{tmdbID}", temporaryShowDetails(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/{mediaType}/{tmdbID}/related", relatedMedia(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/shows/{tmdbID}/seasons/{seasonNumber}", temporaryShowSeasonEpisodes(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/shows/{tmdbID}/seasons/{seasonNumber}/episodes/{episodeNumber}", temporaryEpisodeDetails(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/movies/{tmdbID}", temporaryMovieDetails(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/people/{tmdbID}", personDetails(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/movies/{tmdbID}", mediaDetails(authService, libraryService, metadataProvider, domain.MovieMediaType))
	mux.HandleFunc("GET /api/v1/shows/{tmdbID}", mediaDetails(authService, libraryService, metadataProvider, domain.TVMediaType))
	mux.HandleFunc("GET /api/v1/library", listLibrary(authService, libraryService))
	mux.HandleFunc("POST /api/v1/library", saveLibrary(authService, libraryService, metadataProvider))
	mux.HandleFunc("PATCH /api/v1/library/{mediaID}", updateLibrary(authService, libraryService))
	mux.HandleFunc("PATCH /api/v1/library/{mediaID}/notifications", setLibraryNotifications(authService, libraryService))
	mux.HandleFunc("POST /api/v1/plays", createPlay(authService, trackingService))
	mux.HandleFunc("GET /api/v1/plays", playHistory(authService, trackingService))
	mux.HandleFunc("POST /api/v1/plays/bulk", createBulkPlays(authService, trackingService))
	mux.HandleFunc("DELETE /api/v1/plays/bulk", deleteBulkEpisodePlays(authService, trackingService))
	mux.HandleFunc("DELETE /api/v1/plays/media/{mediaID}", deleteMediaPlays(authService, trackingService))
	mux.HandleFunc("PATCH /api/v1/plays/{playID}", correctPlay(authService, trackingService))
	mux.HandleFunc("DELETE /api/v1/plays/{playID}", deletePlay(authService, trackingService))
	mux.HandleFunc("GET /api/v1/episodes/{episodeID}/rating", episodeRating(authService, trackingService))
	mux.HandleFunc("PUT /api/v1/episodes/{episodeID}/rating", setEpisodeRating(authService, trackingService))
	mux.HandleFunc("GET /api/v1/continue-watching", continueWatching(authService, watchService))
	mux.HandleFunc("GET /api/v1/calendar", calendar(authService, watchService))
	mux.HandleFunc("GET /api/v1/shows/{showID}/progress", showProgress(authService, watchService))
	mux.HandleFunc("GET /api/v1/shows/{showID}/seasons", showSeasons(authService, watchService))
	mux.HandleFunc("GET /api/v1/shows/{showID}/episodes", showEpisodes(authService, watchService))
	mux.HandleFunc("GET /api/v1/seasons/{seasonID}/episodes", seasonEpisodes(authService, watchService))
	if webDir != "" {
		if _, err := os.Stat(webDir); err == nil {
			mux.Handle("GET /", singlePageApp(webDir))
		}
	}
	return &Server{handler: csrfProtection(mux)}
}

func singlePageApp(webDir string) http.Handler {
	files := http.FileServer(http.Dir(webDir))
	index := filepath.Join(webDir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := filepath.Join(webDir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(requested); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}

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

func trending(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		serveTrending(w, r, provider)
	}
}

func publicTrending(provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveTrending(w, r, provider)
	}
}

func relatedMedia(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		relatedProvider, ok := provider.(domain.RelatedMetadataProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "related media metadata is not configured")
			return
		}
		mediaType := domain.MediaType(r.PathValue("mediaType"))
		if mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType {
			writeError(w, http.StatusBadRequest, "media type must be movie or tv")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB ID")
			return
		}
		results, err := relatedProvider.Related(r.Context(), mediaType, tmdbID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "related titles are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, results)
	}
}

func serveTrending(w http.ResponseWriter, r *http.Request, provider domain.MetadataProvider) {
	type response struct {
		TV     []domain.MediaSearchResult `json:"tv"`
		Movies []domain.MediaSearchResult `json:"movies"`
	}
	trendingProvider, ok := provider.(domain.TrendingMetadataProvider)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "TMDB trending is not configured")
		return
	}
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "week"
	}
	if window != "day" && window != "week" {
		writeError(w, http.StatusBadRequest, "window must be day or week")
		return
	}
	movies, err := trendingProvider.Trending(r.Context(), "movie", window)
	if err != nil {
		writeError(w, http.StatusBadGateway, "trending metadata is temporarily unavailable")
		return
	}
	shows, err := trendingProvider.Trending(r.Context(), "tv", window)
	if err != nil {
		writeError(w, http.StatusBadGateway, "trending metadata is temporarily unavailable")
		return
	}
	writeJSON(w, http.StatusOK, response{TV: shows, Movies: movies})
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

func bootstrapStatus(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication is not configured")
			return
		}
		available, err := service.BootstrapAvailable(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "authentication status is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"bootstrap_available": available})
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

func jsonExport(authService *auth.Service, service *exportapp.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "export is not configured")
			return
		}
		data, err := service.Data(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export is temporarily unavailable")
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=visto-export.json")
		writeJSON(w, http.StatusOK, data)
	}
}

func csvExport(authService *auth.Service, service *exportapp.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "export is not configured")
			return
		}
		data, err := service.Data(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export is temporarily unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=visto-export.csv")
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"kind", "id", "media_id", "episode_id", "title", "type", "status", "rating", "watched_at"})
		for _, item := range data.Library {
			rating := ""
			if item.Rating != nil {
				rating = strconv.Itoa(*item.Rating)
			}
			_ = writer.Write([]string{"library", "", item.MediaID, "", item.Title, item.Type, item.Status, rating, ""})
		}
		for _, play := range data.Plays {
			mediaID, episodeID := "", ""
			if play.MediaID != nil {
				mediaID = *play.MediaID
			}
			if play.EpisodeID != nil {
				episodeID = *play.EpisodeID
			}
			_ = writer.Write([]string{"play", play.ID, mediaID, episodeID, "", "", "", "", play.WatchedAt.Format(time.RFC3339Nano)})
		}
		for _, rating := range data.EpisodeRatings {
			value := ""
			if rating.Rating != nil {
				value = strconv.Itoa(*rating.Rating)
			}
			_ = writer.Write([]string{"episode_rating", "", "", rating.EpisodeID, "", "episode", "", value, rating.UpdatedAt.Format(time.RFC3339Nano)})
		}
		writer.Flush()
	}
}

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
			Enabled bool   `json:"enabled"`
			UserKey string `json:"user_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := service.UpdatePushover(r.Context(), user.ID, request.Enabled, request.UserKey); err != nil {
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

func clearPushoverKey(authService *auth.Service, service *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "profile is not configured")
			return
		}
		if err := service.ClearPushoverKey(r.Context(), user.ID); err != nil {
			if errors.Is(err, profile.ErrPushoverUnavailable) {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "Pushover key could not be removed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func createUser(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		if actor.Role != domain.AdminRole {
			writeError(w, http.StatusForbidden, "administrator access required")
			return
		}
		var request credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, err := service.CreateUser(r.Context(), request.Username, request.DisplayName, request.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user)
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

type temporaryShowDetailsResponse struct {
	Media   library.Media         `json:"media"`
	Seasons []temporaryShowSeason `json:"seasons"`
	Cast    []domain.TVCastMember `json:"cast"`
}

type temporaryShowSeason struct {
	TMDBID     int64               `json:"tmdb_id"`
	Number     int                 `json:"season_number"`
	Name       string              `json:"name"`
	Overview   string              `json:"overview,omitempty"`
	PosterPath string              `json:"poster_path,omitempty"`
	AirDate    string              `json:"air_date,omitempty"`
	Episodes   []watch.ShowEpisode `json:"episodes"`
}

type temporaryMovieDetailsResponse struct {
	Media       library.Media         `json:"media"`
	Runtime     int                   `json:"runtime,omitempty"`
	VoteAverage float32               `json:"vote_average,omitempty"`
	Genres      []string              `json:"genres,omitempty"`
	Cast        []domain.TVCastMember `json:"cast"`
}

type temporaryEpisodeDetailsResponse struct {
	Name           string                `json:"name"`
	Overview       string                `json:"overview,omitempty"`
	AirDate        string                `json:"air_date,omitempty"`
	Runtime        int                   `json:"runtime,omitempty"`
	StillPath      string                `json:"still_path,omitempty"`
	VoteAverage    float32               `json:"vote_average,omitempty"`
	ProductionCode string                `json:"production_code,omitempty"`
	GuestStars     []domain.TVCastMember `json:"guest_stars,omitempty"`
	Crew           []domain.TVCrewMember `json:"crew,omitempty"`
}

func temporaryShowDetails(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		tvProvider, ok := provider.(domain.TVShowSummaryProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "TV summary metadata is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB ID")
			return
		}
		show, err := tvProvider.ShowSummary(r.Context(), tmdbID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "TV details are temporarily unavailable")
			return
		}
		response := temporaryShowDetailsResponse{
			Media: library.Media{ID: fmt.Sprintf("tv:%d", show.TMDBID), Type: domain.TVMediaType, TMDBID: show.TMDBID, Title: show.Name, OriginalTitle: show.Name, Overview: show.Overview, ReleaseDate: show.FirstAirDate, PosterPath: show.PosterPath, BackdropPath: show.BackdropPath, OriginalLanguage: show.OriginalLanguage, Status: show.Status},
			Cast:  show.Cast,
		}
		for _, season := range show.Seasons {
			response.Seasons = append(response.Seasons, temporaryShowSeason{TMDBID: season.TMDBID, Number: season.Number, Name: season.Name, Overview: season.Overview, PosterPath: season.PosterPath, AirDate: season.AirDate, Episodes: nil})
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func temporaryShowSeasonEpisodes(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		tvProvider, ok := provider.(domain.TVShowSummaryProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "TV season metadata is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		seasonNumber, seasonErr := strconv.Atoi(r.PathValue("seasonNumber"))
		if err != nil || tmdbID <= 0 || seasonErr != nil || seasonNumber < 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB show or season")
			return
		}
		season, err := tvProvider.Season(r.Context(), tmdbID, seasonNumber)
		if err != nil {
			writeError(w, http.StatusBadGateway, "TV season details are temporarily unavailable")
			return
		}
		response := temporaryShowSeason{TMDBID: season.TMDBID, Number: season.Number, Name: season.Name, Overview: season.Overview, PosterPath: season.PosterPath, AirDate: season.AirDate, Episodes: []watch.ShowEpisode{}}
		for _, episode := range season.Episodes {
			var airDate *time.Time
			if parsed, parseErr := time.Parse(time.DateOnly, episode.AirDate); parseErr == nil {
				airDate = &parsed
			}
			response.Episodes = append(response.Episodes, watch.ShowEpisode{Episode: domain.Episode{ID: fmt.Sprintf("tv:%d:episode:%d", tmdbID, episode.TMDBID), ShowID: fmt.Sprintf("tv:%d", tmdbID), SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber, AirDate: airDate}, Name: episode.Name, Overview: episode.Overview, Runtime: episode.Runtime, StillPath: episode.StillPath})
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func temporaryMovieDetails(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		movieProvider, ok := provider.(domain.MovieMetadataProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "movie metadata is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB ID")
			return
		}
		movie, err := movieProvider.Movie(r.Context(), tmdbID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "movie details are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, temporaryMovieDetailsResponse{
			Media:   library.Media{ID: fmt.Sprintf("movie:%d", movie.TMDBID), Type: domain.MovieMediaType, TMDBID: movie.TMDBID, Title: movie.Title, OriginalTitle: movie.OriginalTitle, Overview: movie.Overview, ReleaseDate: movie.ReleaseDate, PosterPath: movie.PosterPath, BackdropPath: movie.BackdropPath, OriginalLanguage: movie.OriginalLanguage, Status: movie.Status},
			Runtime: movie.Runtime, VoteAverage: movie.VoteAverage, Genres: movie.Genres, Cast: movie.Cast,
		})
	}
}

func personDetails(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		personProvider, ok := provider.(domain.PersonMetadataProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "person metadata is not configured")
			return
		}
		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil || tmdbID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TMDB person ID")
			return
		}
		person, err := personProvider.Person(r.Context(), tmdbID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "person details are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, person)
	}
}

func temporaryEpisodeDetails(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		episodeProvider, ok := provider.(domain.TVEpisodeMetadataProvider)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "episode metadata is not configured")
			return
		}
		showID, showErr := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		seasonNumber, seasonErr := strconv.Atoi(r.PathValue("seasonNumber"))
		episodeNumber, episodeErr := strconv.Atoi(r.PathValue("episodeNumber"))
		if showErr != nil || showID <= 0 || seasonErr != nil || seasonNumber < 0 || episodeErr != nil || episodeNumber <= 0 {
			writeError(w, http.StatusBadRequest, "invalid TV episode")
			return
		}
		episode, err := episodeProvider.Episode(r.Context(), showID, seasonNumber, episodeNumber)
		if err != nil {
			writeError(w, http.StatusBadGateway, "episode details are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, temporaryEpisodeDetailsResponse{Name: episode.Name, Overview: episode.Overview, AirDate: episode.AirDate, Runtime: episode.Runtime, StillPath: episode.StillPath, VoteAverage: episode.VoteAverage, ProductionCode: episode.ProductionCode, GuestStars: episode.GuestStars, Crew: episode.Crew})
	}
}

func search(authService *auth.Service, provider domain.MetadataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		if provider == nil {
			writeError(w, http.StatusServiceUnavailable, "metadata search is not configured")
			return
		}
		query := r.URL.Query().Get("q")
		if query == "" {
			writeError(w, http.StatusBadRequest, "q is required")
			return
		}
		results, err := provider.Search(r.Context(), query, r.URL.Query().Get("language"))
		if err != nil {
			writeError(w, http.StatusBadGateway, "metadata search is temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, results)
	}
}

type credentialsRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

func bootstrap(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, err := service.Bootstrap(r.Context(), request.Username, request.DisplayName, request.Password)
		if err != nil {
			if err == auth.ErrBootstrapComplete {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user)
	}
}
func login(service *auth.Service, limiter *LoginLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if allowed, retryAfter := limiter.allowed(ip); !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
			writeError(w, http.StatusTooManyRequests, "too many login attempts")
			return
		}
		var request credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, token, expiresAt, err := service.Login(r.Context(), request.Username, request.Password)
		if err != nil {
			limiter.failed(ip)
			writeError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		limiter.reset(ip)
		http.SetCookie(w, &http.Cookie{Name: "visto_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expiresAt, Secure: cookieSecure(r)})
		writeJSON(w, http.StatusOK, user)
	}
}

func logout(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication is not configured")
			return
		}
		if cookie, err := r.Cookie("visto_session"); err == nil {
			if err := service.Logout(r.Context(), cookie.Value); err != nil {
				writeError(w, http.StatusInternalServerError, "could not end session")
				return
			}
		}
		http.SetCookie(w, &http.Cookie{Name: "visto_session", Value: "", Path: "/", HttpOnly: true, Secure: cookieSecure(r), SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1})
		w.WriteHeader(http.StatusNoContent)
	}
}

func cookieSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func currentUser(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, user)
	}
}

func authenticatedUser(w http.ResponseWriter, r *http.Request, service *auth.Service) (domain.User, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Add("Vary", "Cookie")
	w.Header().Add("Vary", "Authorization")
	if service == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return domain.User{}, false
	}
	var user domain.User
	var err error
	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return domain.User{}, false
		}
		user, err = service.AuthenticatePersonalToken(r.Context(), parts[1])
	} else {
		cookie, cookieErr := r.Cookie("visto_session")
		if cookieErr != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return domain.User{}, false
		}
		user, err = service.Authenticate(r.Context(), cookie.Value)
	}
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return domain.User{}, false
	}
	return user, true
}

func listPersonalTokens(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		tokens, err := service.PersonalTokens(r.Context(), user.ID)
		if err != nil {
			status := http.StatusInternalServerError
			message := "could not list personal API tokens"
			if errors.Is(err, auth.ErrPersonalTokensUnavailable) {
				status = http.StatusServiceUnavailable
				message = "personal API tokens are not configured"
			}
			writeError(w, status, message)
			return
		}
		writeJSON(w, http.StatusOK, tokens)
	}
}

func createPersonalToken(service *auth.Service) http.HandlerFunc {
	type requestBody struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		var body requestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		var expiresAt *time.Time
		if strings.TrimSpace(body.ExpiresAt) != "" {
			parsed, err := time.Parse(time.RFC3339, body.ExpiresAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "expires_at must be an RFC 3339 timestamp")
				return
			}
			expiresAt = &parsed
		}
		token, err := service.CreatePersonalToken(r.Context(), user.ID, body.Name, expiresAt)
		if err != nil {
			status := http.StatusInternalServerError
			message := "could not create personal API token"
			if errors.Is(err, auth.ErrInvalidPersonalToken) {
				status = http.StatusBadRequest
				message = err.Error()
			} else if errors.Is(err, auth.ErrPersonalTokensUnavailable) {
				status = http.StatusServiceUnavailable
				message = "personal API tokens are not configured"
			}
			writeError(w, status, message)
			return
		}
		writeJSON(w, http.StatusCreated, token)
	}
}

func revokePersonalToken(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		if err := service.RevokePersonalToken(r.Context(), user.ID, r.PathValue("tokenID")); err != nil {
			if errors.Is(err, auth.ErrPersonalTokenMissing) {
				writeError(w, http.StatusNotFound, "personal API token not found")
				return
			}
			status := http.StatusInternalServerError
			if errors.Is(err, auth.ErrPersonalTokensUnavailable) {
				status = http.StatusServiceUnavailable
			}
			writeError(w, status, "could not revoke personal API token")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (server *Server) Handler() http.Handler {
	return server.handler
}

func health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}
