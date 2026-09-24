package httpserver

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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
	mux.HandleFunc("GET /api/v1/profile/activity-settings", activitySettings(authService, profileService))
	mux.HandleFunc("PATCH /api/v1/profile/activity-settings", setActivitySettings(authService, profileService))
	mux.HandleFunc("GET /api/v1/feed", instanceFeed(authService, feedService))
	mux.HandleFunc("GET /api/v1/export/json", jsonExport(authService, exportService))
	mux.HandleFunc("GET /api/v1/export/csv", csvExport(authService, exportService))
	mux.HandleFunc("GET /api/v1/search", search(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/library", listLibrary(authService, libraryService))
	mux.HandleFunc("POST /api/v1/library", saveLibrary(authService, libraryService, metadataProvider))
	mux.HandleFunc("PATCH /api/v1/library/{mediaID}", updateLibrary(authService, libraryService))
	mux.HandleFunc("POST /api/v1/plays", createPlay(authService, trackingService))
	mux.HandleFunc("GET /api/v1/plays", playHistory(authService, trackingService))
	mux.HandleFunc("POST /api/v1/plays/bulk", createBulkPlays(authService, trackingService))
	mux.HandleFunc("PATCH /api/v1/plays/{playID}", correctPlay(authService, trackingService))
	mux.HandleFunc("DELETE /api/v1/plays/{playID}", deletePlay(authService, trackingService))
	mux.HandleFunc("GET /api/v1/continue-watching", continueWatching(authService, watchService))
	mux.HandleFunc("GET /api/v1/calendar", calendar(authService, watchService))
	if webDir != "" {
		if _, err := os.Stat(webDir); err == nil {
			mux.Handle("GET /", http.FileServer(http.Dir(webDir)))
		}
	}
	return &Server{handler: csrfProtection(mux)}
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
	cookie, err := r.Cookie("visto_session")
	if err != nil || service == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return domain.User{}, false
	}
	user, err := service.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return domain.User{}, false
	}
	return user, true
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
