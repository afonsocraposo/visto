package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/oauth"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/presentation/security"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

const protocolVersion = "2025-11-25"

const currentProtocolVersion = "2026-07-28"

type authenticator interface {
	AuthenticatePersonalToken(context.Context, string) (domain.User, error)
}

type libraryUseCases interface {
	Save(context.Context, string, string, domain.LibraryStatus, *int) (library.Item, error)
	SaveMedia(context.Context, string, library.Media, domain.LibraryStatus, *int) (library.Item, error)
	List(context.Context, string) ([]library.Entry, error)
	ImportShow(context.Context, int64, domain.TVShowMetadataProvider) error
}

type trackingUseCases interface {
	Record(context.Context, string, *string, *string, time.Time, string) (tracking.Play, error)
	History(context.Context, string, int) ([]tracking.HistoryEntry, error)
	RateEpisode(context.Context, string, string, *int) (tracking.EpisodeRating, error)
}

type watchUseCases interface {
	Continue(context.Context, string) ([]watch.ContinueEntry, error)
	Calendar(context.Context, string, time.Time, time.Time) ([]watch.CalendarEntry, error)
	ShowProgress(context.Context, string, string) (watch.Progress, error)
}

type searchUseCase interface {
	Search(context.Context, string, string) ([]domain.MediaSearchResult, error)
}

type Server struct {
	auth        authenticator
	credentials *auth.Service
	oauth       *oauth.Service
	publicURL   string
	library     libraryUseCases
	tracking    trackingUseCases
	watch       watchUseCases
	metadata    domain.MetadataProvider
	proxies     security.ProxyResolver
	rateLimiter *security.RateLimiter
	mcpOnce     sync.Once
	mcpHandler  http.Handler
}

func New(authService *auth.Service, oauthService *oauth.Service, publicURL string, metadata domain.MetadataProvider, libraryService *library.Service, trackingService *tracking.Service, watchService *watch.Service) http.Handler {
	return NewWithTrustedProxies(authService, oauthService, publicURL, metadata, libraryService, trackingService, watchService, security.ProxyResolver{})
}

func NewWithTrustedProxies(authService *auth.Service, oauthService *oauth.Service, publicURL string, metadata domain.MetadataProvider, libraryService *library.Service, trackingService *tracking.Service, watchService *watch.Service, proxies security.ProxyResolver) http.Handler {
	server := &Server{auth: authService, credentials: authService, oauth: oauthService, publicURL: strings.TrimRight(publicURL, "/"), metadata: metadata, library: libraryService, tracking: trackingService, watch: watchService, proxies: proxies, rateLimiter: security.NewRateLimiter(10_000)}
	return server.Handler()
}

func (server *Server) Handler() http.Handler {
	return http.HandlerFunc(server.routeHTTP)
}

func (server *Server) routeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/mcp":
		server.sdkHTTPHandler().ServeHTTP(w, r)
	case "/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp":
		server.protectedResourceMetadata(w, r)
	case "/.well-known/oauth-authorization-server":
		server.authorizationServerMetadata(w, r)
	case "/oauth/register":
		server.registerOAuthClient(w, r)
	case "/oauth/authorize":
		server.authorizeOAuthClient(w, r)
	case "/oauth/token":
		server.issueOAuthToken(w, r)
	case "/oauth/revoke":
		server.revokeOAuthToken(w, r)
	default:
		http.NotFound(w, r)
	}
}

type mcpIdentity struct {
	userID        string
	scopes        map[string]bool
	personalToken bool
}

type mcpIdentityKey struct{}

func (server *Server) sdkHTTPHandler() http.Handler {
	server.mcpOnce.Do(func() {
		mcpServer := mcpgo.NewServer(&mcpgo.Implementation{Name: "visto", Title: "Visto", Version: "0.3.0", Description: "Access the authenticated user's Visto library."}, nil)
		for _, definition := range toolDefinitions() {
			definition := definition
			mcpServer.AddTool(&mcpgo.Tool{Name: definition.Name, Description: definition.Description, InputSchema: definition.InputSchema, Annotations: sdkToolAnnotations(definition.Annotations), Meta: mcpgo.Meta{"securitySchemes": definition.SecuritySchemes}}, server.sdkToolHandler(definition.Name))
		}
		var handler http.Handler = mcpgo.NewStreamableHTTPHandler(func(*http.Request) *mcpgo.Server { return mcpServer }, &mcpgo.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 1 << 20, PropagateRequestCancellation: true})
		handler = toolDiscoveryHandler(handler)
		server.mcpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validOrigin(r) {
				http.Error(w, "Origin is not allowed", http.StatusForbidden)
				return
			}
			identity, ok := server.authenticateMCP(r)
			if !ok {
				server.writeOAuthChallenge(w, "read", "unauthorized")
				http.Error(w, "A Visto access token is required", http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), mcpIdentityKey{}, identity)))
		})
	})
	return server.mcpHandler
}

// The SDK emits securitySchemes only in _meta. OpenAI clients also read the
// root-level field during action discovery, so mirror it on tools/list.
func toolDiscoveryHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		prefix, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
		r.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(prefix), r.Body), r.Body}
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		var request struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(prefix, &request) != nil || request.Method != "tools/list" {
			next.ServeHTTP(w, r)
			return
		}
		response := httptest.NewRecorder()
		next.ServeHTTP(response, r)
		body := response.Body.Bytes()
		if response.Code == http.StatusOK {
			var payload map[string]any
			if json.Unmarshal(body, &payload) == nil {
				if result, ok := payload["result"].(map[string]any); ok {
					if tools, ok := result["tools"].([]any); ok {
						for _, item := range tools {
							tool, ok := item.(map[string]any)
							if !ok {
								continue
							}
							if meta, ok := tool["_meta"].(map[string]any); ok {
								tool["securitySchemes"] = meta["securitySchemes"]
							}
						}
						if encoded, err := json.Marshal(payload); err == nil {
							body = encoded
						}
					}
				}
			}
		}
		for key, values := range response.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.Header().Del("Content-Length")
		w.WriteHeader(response.Code)
		_, _ = w.Write(body)
	})
}

func (server *Server) authenticateMCP(r *http.Request) (mcpIdentity, bool) {
	token := bearerToken(r.Header.Get("Authorization"))
	if server.oauth != nil && token != "" {
		identity, err := server.oauth.Authenticate(r.Context(), token, server.mcpResource())
		if err == nil {
			scopes := make(map[string]bool, len(identity.Scopes))
			for _, scope := range identity.Scopes {
				scopes[scope] = true
			}
			return mcpIdentity{userID: identity.UserID, scopes: scopes}, true
		}
	}
	if server.auth != nil && token != "" {
		user, err := server.auth.AuthenticatePersonalToken(r.Context(), token)
		if err == nil {
			return mcpIdentity{userID: user.ID, personalToken: true}, true
		}
	}
	return mcpIdentity{}, false
}

func (server *Server) sdkToolHandler(name string) mcpgo.ToolHandler {
	return func(ctx context.Context, request *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		identity, ok := ctx.Value(mcpIdentityKey{}).(mcpIdentity)
		if !ok || identity.userID == "" {
			return sdkToolError("A Visto access token is required"), nil
		}
		if scope := requiredScope(name); scope != "" && !identity.personalToken && !identity.scopes[scope] {
			return &mcpgo.CallToolResult{IsError: true, Content: []mcpgo.Content{&mcpgo.TextContent{Text: "This Visto action needs additional permission. Reconnect and approve the requested scope."}}, Meta: mcpgo.Meta{"mcp/www_authenticate": []string{server.scopeChallenge(scope)}}}, nil
		}
		var arguments map[string]any
		if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
			return sdkToolError("Invalid tool call parameters"), nil
		}
		result, err := server.callTool(ctx, identity.userID, name, arguments)
		if err != nil {
			return sdkToolError(err.Error()), nil
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return sdkToolError("Could not encode the tool result"), nil
		}
		return &mcpgo.CallToolResult{Content: []mcpgo.Content{&mcpgo.TextContent{Text: string(encoded)}}, StructuredContent: result}, nil
	}
}

func sdkToolError(message string) *mcpgo.CallToolResult {
	return &mcpgo.CallToolResult{IsError: true, Content: []mcpgo.Content{&mcpgo.TextContent{Text: message}}}
}

func sdkToolAnnotations(annotations map[string]any) *mcpgo.ToolAnnotations {
	readOnly, _ := annotations["readOnlyHint"].(bool)
	destructive, _ := annotations["destructiveHint"].(bool)
	openWorld, _ := annotations["openWorldHint"].(bool)
	return &mcpgo.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: &destructive, OpenWorldHint: &openWorld}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func requiredScope(tool string) string {
	switch tool {
	case "search_media", "get_currently_watching", "get_show_progress", "get_upcoming_episodes", "get_watch_history":
		return oauth.ReadScope
	case "add_to_watchlist", "set_show_status", "mark_movie_watched", "mark_episode_watched", "rate_media":
		return oauth.WriteScope
	default:
		return ""
	}
}

func (server *Server) scopeChallenge(scope string) string {
	challenge := fmt.Sprintf(`Bearer scope=%q, error="insufficient_scope", error_description=%q`, scope, "Additional permission is required")
	if metadata := server.resourceMetadataURL(); metadata != "" {
		challenge = fmt.Sprintf(`Bearer resource_metadata=%q, scope=%q, error="insufficient_scope", error_description=%q`, metadata, scope, "Additional permission is required")
	}
	return challenge
}

func validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && strings.EqualFold(parsed.Host, r.Host)
}
