package httpserver

import "net/http"
import "os"
import "path/filepath"
import "time"
import "github.com/afonsocosta/visto/internal/application/auth"
import exportapp "github.com/afonsocosta/visto/internal/application/export"
import "github.com/afonsocosta/visto/internal/application/feed"
import "github.com/afonsocosta/visto/internal/application/library"
import "github.com/afonsocosta/visto/internal/application/oauth"
import "github.com/afonsocosta/visto/internal/application/profile"
import "github.com/afonsocosta/visto/internal/application/tracking"
import "github.com/afonsocosta/visto/internal/application/watch"
import "github.com/afonsocosta/visto/internal/domain"
import "github.com/afonsocosta/visto/internal/presentation/security"

type Server struct {
	handler     http.Handler
	mux         *http.ServeMux
	authService *auth.Service
	proxies     *security.ProxyResolver
}

func New(authService *auth.Service, metadataProvider domain.MetadataProvider, webDir string, libraryService *library.Service, trackingService *tracking.Service, profileService *profile.Service, feedService *feed.Service, exportService *exportapp.Service, watchService *watch.Service) *Server {
	mux := http.NewServeMux()
	proxies := &security.ProxyResolver{}
	loginLimiter := newLoginLimiter()
	registerAPIRoutes(mux, authService, metadataProvider, libraryService, trackingService, profileService, feedService, exportService, watchService, loginLimiter, proxies)
	if webDir != "" {
		if _, err := os.Stat(webDir); err == nil {
			mux.Handle("GET /", singlePageApp(webDir))
		}
	}
	return &Server{handler: csrfProtection(mux, proxies), mux: mux, authService: authService, proxies: proxies}
}

func (server *Server) WithTrustedProxies(proxies security.ProxyResolver) *Server {
	if server.proxies != nil {
		*server.proxies = proxies
	}
	return server
}

// WithOAuth adds account-management endpoints for OAuth clients. It is kept
// optional so Visto can run without an OAuth configuration.
func (server *Server) WithOAuth(service *oauth.Service) *Server {
	server.mux.HandleFunc("GET /api/v1/connected-apps", listConnectedApps(server.authService, service))
	server.mux.HandleFunc("DELETE /api/v1/connected-apps", revokeAllConnectedApps(server.authService, service))
	server.mux.HandleFunc("DELETE /api/v1/connected-apps/{clientID}", revokeConnectedApp(server.authService, service))
	return server
}

// WithGoogleOAuth adds optional Google sign-in routes when all credentials are set.
func (server *Server) WithGoogleOAuth(config GoogleOAuthConfig) *Server {
	if config.Enabled() {
		server.mux.HandleFunc("GET /api/v1/auth/google", googleLogin(config, server.authService, server.proxies))
		server.mux.HandleFunc("GET /api/v1/auth/google/callback", googleCallback(config, server.authService, server.proxies))
	}
	return server
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

func listConnectedApps(authService *auth.Service, service *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "connected apps are not configured")
			return
		}
		connections, err := service.Connections(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list connected apps")
			return
		}
		writeJSON(w, http.StatusOK, connections)
	}
}

func revokeConnectedApp(authService *auth.Service, service *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "connected apps are not configured")
			return
		}
		if err := service.RevokeConnection(r.Context(), user.ID, r.PathValue("clientID")); err != nil {
			writeError(w, http.StatusBadRequest, "could not revoke connected app")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func revokeAllConnectedApps(authService *auth.Service, service *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "connected apps are not configured")
			return
		}
		if err := service.RevokeAllConnections(r.Context(), user.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not revoke connected apps")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
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
