package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
)

type Server struct {
	handler http.Handler
}

func New(authService *auth.Service, metadataProvider domain.MetadataProvider) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", bootstrap(authService))
	mux.HandleFunc("POST /api/v1/auth/login", login(authService))
	mux.HandleFunc("GET /api/v1/me", currentUser(authService))
	mux.HandleFunc("GET /api/v1/search", search(authService, metadataProvider))
	return &Server{handler: mux}
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
