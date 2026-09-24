package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
)

type Server struct {
	handler http.Handler
}

func New(authService *auth.Service, metadataProvider domain.MetadataProvider, webDir string, libraryServices ...*library.Service) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", bootstrap(authService))
	mux.HandleFunc("POST /api/v1/auth/login", login(authService))
	mux.HandleFunc("GET /api/v1/me", currentUser(authService))
	mux.HandleFunc("GET /api/v1/search", search(authService, metadataProvider))
	var libraryService *library.Service
	if len(libraryServices) > 0 {
		libraryService = libraryServices[0]
	}
	mux.HandleFunc("GET /api/v1/library", listLibrary(authService, libraryService))
	mux.HandleFunc("POST /api/v1/library", saveLibrary(authService, libraryService))
	if webDir != "" {
		if _, err := os.Stat(webDir); err == nil {
			mux.Handle("GET /", http.FileServer(http.Dir(webDir)))
		}
	}
	return &Server{handler: mux}
}

type libraryRequest struct {
	Media  library.Media        `json:"media"`
	Status domain.LibraryStatus `json:"status"`
	Rating *int                 `json:"rating"`
}

func saveLibrary(authService *auth.Service, service *library.Service) http.HandlerFunc {
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
func login(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, token, expiresAt, err := service.Login(r.Context(), request.Username, request.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "visto_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expiresAt, Secure: r.TLS != nil})
		writeJSON(w, http.StatusOK, user)
	}
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
