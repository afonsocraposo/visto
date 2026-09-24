// Package mcp exposes Visto application use cases through the Model Context
// Protocol. It is a presentation adapter and does not call the REST API.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
		handler := mcpgo.NewStreamableHTTPHandler(func(*http.Request) *mcpgo.Server { return mcpServer }, &mcpgo.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 1 << 20, PropagateRequestCancellation: true})
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

type toolDefinition struct {
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	InputSchema     map[string]any   `json:"inputSchema"`
	Annotations     map[string]any   `json:"annotations,omitempty"`
	SecuritySchemes []map[string]any `json:"securitySchemes,omitempty"`
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required}
}

func toolDefinitions() []toolDefinition {
	readOnly := map[string]any{"readOnlyHint": true, "destructiveHint": false, "openWorldHint": true}
	writeAction := map[string]any{"readOnlyHint": false, "destructiveHint": false, "openWorldHint": false}
	readSecurity := []map[string]any{{"type": "oauth2", "scopes": []string{oauth.ReadScope}}}
	writeSecurity := []map[string]any{{"type": "oauth2", "scopes": []string{oauth.WriteScope}}}
	return []toolDefinition{
		{Name: "search_media", Description: "Search TMDB for movies and TV shows.", InputSchema: objectSchema(map[string]any{"query": map[string]string{"type": "string"}, "language": map[string]string{"type": "string"}}, "query"), Annotations: readOnly, SecuritySchemes: readSecurity},
		{Name: "get_currently_watching", Description: "List the next unwatched episode for each show the user is watching.", InputSchema: objectSchema(map[string]any{}), Annotations: readOnly, SecuritySchemes: readSecurity},
		{Name: "get_show_progress", Description: "Get watched and next-episode progress for a tracked TV show. Use a Visto show ID such as tv:123.", InputSchema: objectSchema(map[string]any{"show_id": map[string]string{"type": "string"}}, "show_id"), Annotations: readOnly, SecuritySchemes: readSecurity},
		{Name: "get_upcoming_episodes", Description: "List upcoming unwatched episodes for shows in the user's watching list.", InputSchema: objectSchema(map[string]any{"from": map[string]string{"type": "string", "format": "date"}, "to": map[string]string{"type": "string", "format": "date"}}), Annotations: readOnly, SecuritySchemes: readSecurity},
		{Name: "get_watch_history", Description: "Read recent watch history for the authenticated user.", InputSchema: objectSchema(map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 500}}), Annotations: readOnly, SecuritySchemes: readSecurity},
		{Name: "add_to_watchlist", Description: "Save a movie or TV show to the user's watchlist. Pass a media result returned by search_media.", InputSchema: objectSchema(map[string]any{"media": searchResultSchema()}, "media"), Annotations: writeAction, SecuritySchemes: writeSecurity},
		{Name: "set_show_status", Description: "Change a tracked TV show's library status.", InputSchema: objectSchema(map[string]any{"show_id": map[string]string{"type": "string"}, "status": statusSchema()}, "show_id", "status"), Annotations: writeAction, SecuritySchemes: writeSecurity},
		{Name: "mark_movie_watched", Description: "Mark a movie watched. Pass a media result returned by search_media.", InputSchema: objectSchema(map[string]any{"media": searchResultSchema(), "watched_at": map[string]string{"type": "string", "format": "date-time"}}, "media"), Annotations: writeAction, SecuritySchemes: writeSecurity},
		{Name: "mark_episode_watched", Description: "Mark one episode watched. Adds the show to Watching if needed. Does not mark prior episodes automatically. Pass a show result returned by search_media and the Visto episode ID.", InputSchema: objectSchema(map[string]any{"show": searchResultSchema(), "episode_id": map[string]string{"type": "string"}, "watched_at": map[string]string{"type": "string", "format": "date-time"}}, "show", "episode_id"), Annotations: writeAction, SecuritySchemes: writeSecurity},
		{Name: "rate_media", Description: "Set a 1–5 star rating, or pass null to clear it, for a tracked movie, TV show, or episode.", InputSchema: objectSchema(map[string]any{"media_id": map[string]string{"type": "string"}, "episode_id": map[string]string{"type": "string"}, "rating": map[string]any{"type": []string{"integer", "null"}, "minimum": 1, "maximum": 5}}, "rating"), Annotations: writeAction, SecuritySchemes: writeSecurity},
	}
}

func searchResultSchema() map[string]any {
	return objectSchema(map[string]any{
		"tmdb_id": map[string]any{"type": "integer", "minimum": 1}, "type": map[string]any{"type": "string", "enum": []string{"movie", "tv"}},
		"title": map[string]string{"type": "string"}, "original_title": map[string]string{"type": "string"}, "overview": map[string]string{"type": "string"},
		"release_date": map[string]string{"type": "string"}, "poster_path": map[string]string{"type": "string"}, "backdrop_path": map[string]string{"type": "string"}, "original_language": map[string]string{"type": "string"},
	}, "tmdb_id", "type", "title")
}

func statusSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"watchlist", "watching", "paused", "dropped"}}
}

func (server *Server) callTool(ctx context.Context, userID, name string, args map[string]any) (any, error) {
	switch name {
	case "search_media":
		provider, ok := server.metadata.(searchUseCase)
		if !ok {
			return nil, fmt.Errorf("TMDB search is not configured")
		}
		query := stringArg(args, "query")
		if query == "" {
			return nil, fmt.Errorf("query is required")
		}
		return provider.Search(ctx, query, stringArg(args, "language"))
	case "get_currently_watching":
		if server.watch == nil {
			return nil, fmt.Errorf("watch progress is not configured")
		}
		return server.watch.Continue(ctx, userID)
	case "get_show_progress":
		if server.watch == nil {
			return nil, fmt.Errorf("watch progress is not configured")
		}
		showID := stringArg(args, "show_id")
		if !strings.HasPrefix(showID, "tv:") {
			return nil, fmt.Errorf("show_id must be a Visto TV ID such as tv:123")
		}
		return server.watch.ShowProgress(ctx, userID, showID)
	case "get_upcoming_episodes":
		if server.watch == nil {
			return nil, fmt.Errorf("watch calendar is not configured")
		}
		from, err := dateArg(args, "from")
		if err != nil {
			return nil, err
		}
		to, err := dateArg(args, "to")
		if err != nil {
			return nil, err
		}
		return server.watch.Calendar(ctx, userID, from, to)
	case "get_watch_history":
		if server.tracking == nil {
			return nil, fmt.Errorf("watch history is not configured")
		}
		limit := intArg(args, "limit")
		return server.tracking.History(ctx, userID, limit)
	case "add_to_watchlist":
		media, err := mediaArg(args)
		if err != nil {
			return nil, err
		}
		item, err := server.saveMedia(ctx, userID, media, domain.WatchlistStatus)
		if err != nil {
			return nil, err
		}
		return item, nil
	case "set_show_status":
		if server.library == nil {
			return nil, fmt.Errorf("library is not configured")
		}
		showID, status := stringArg(args, "show_id"), domain.LibraryStatus(stringArg(args, "status"))
		if !strings.HasPrefix(showID, "tv:") {
			return nil, fmt.Errorf("show_id must be a Visto TV ID such as tv:123")
		}
		if status != domain.WatchlistStatus && status != domain.WatchingStatus && status != domain.PausedStatus && status != domain.DroppedStatus {
			return nil, fmt.Errorf("invalid show status")
		}
		return server.library.Save(ctx, userID, showID, status, nil)
	case "mark_movie_watched":
		if server.tracking == nil {
			return nil, fmt.Errorf("watch tracking is not configured")
		}
		media, err := mediaArg(args)
		if err != nil {
			return nil, err
		}
		if media.Type != domain.MovieMediaType {
			return nil, fmt.Errorf("media must be a movie")
		}
		if _, err := server.saveMedia(ctx, userID, media, domain.WatchingStatus); err != nil {
			return nil, err
		}
		mediaID := fmt.Sprintf("movie:%d", media.TMDBID)
		watchedAt, err := timestampArg(args, "watched_at")
		if err != nil {
			return nil, err
		}
		return server.tracking.Record(ctx, userID, &mediaID, nil, watchedAt, "mcp")
	case "mark_episode_watched":
		if server.tracking == nil {
			return nil, fmt.Errorf("watch tracking is not configured")
		}
		show, err := mediaArg(map[string]any{"media": args["show"]})
		if err != nil {
			return nil, fmt.Errorf("show: %w", err)
		}
		if show.Type != domain.TVMediaType {
			return nil, fmt.Errorf("show must be a TV search result")
		}
		episodeID := stringArg(args, "episode_id")
		showID := fmt.Sprintf("tv:%d", show.TMDBID)
		if episodeID == "" || !strings.HasPrefix(episodeID, showID+":episode:") {
			return nil, fmt.Errorf("episode_id must be a Visto episode ID")
		}
		if err := server.ensureTrackedShow(ctx, userID, show); err != nil {
			return nil, err
		}
		watchedAt, err := timestampArg(args, "watched_at")
		if err != nil {
			return nil, err
		}
		return server.tracking.Record(ctx, userID, nil, &episodeID, watchedAt, "mcp")
	case "rate_media":
		if server.library == nil || server.tracking == nil {
			return nil, fmt.Errorf("ratings are not configured")
		}
		rating, err := ratingArg(args)
		if err != nil {
			return nil, err
		}
		if episodeID := stringArg(args, "episode_id"); episodeID != "" {
			return server.tracking.RateEpisode(ctx, userID, episodeID, rating)
		}
		mediaID := stringArg(args, "media_id")
		if mediaID == "" {
			return nil, fmt.Errorf("media_id or episode_id is required")
		}
		entries, err := server.library.List(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.Item.MediaID == mediaID {
				return server.library.Save(ctx, userID, mediaID, entry.Item.Status, rating)
			}
		}
		return nil, fmt.Errorf("media is not in the user's library")
	default:
		return nil, fmt.Errorf("unknown Visto tool %q", name)
	}
}

func (server *Server) saveMedia(ctx context.Context, userID string, item domain.MediaSearchResult, status domain.LibraryStatus) (library.Item, error) {
	if server.library == nil {
		return library.Item{}, fmt.Errorf("library is not configured")
	}
	if item.Type != domain.MovieMediaType && item.Type != domain.TVMediaType {
		return library.Item{}, fmt.Errorf("media type must be movie or tv")
	}
	media := library.Media{TMDBID: item.TMDBID, Type: item.Type, Title: item.Title, OriginalTitle: item.OriginalTitle, Overview: item.Overview, ReleaseDate: item.ReleaseDate, PosterPath: item.PosterPath, BackdropPath: item.BackdropPath, OriginalLanguage: item.OriginalLanguage}
	entry, err := server.library.SaveMedia(ctx, userID, media, status, nil)
	if err != nil {
		return library.Item{}, err
	}
	if item.Type == domain.TVMediaType {
		provider, ok := server.metadata.(domain.TVShowMetadataProvider)
		if !ok {
			return library.Item{}, fmt.Errorf("TV show metadata is not configured")
		}
		if err := server.library.ImportShow(ctx, item.TMDBID, provider); err != nil {
			return library.Item{}, fmt.Errorf("show was saved but episode catalog import failed: %w", err)
		}
	}
	return entry, nil
}

func (server *Server) ensureTrackedShow(ctx context.Context, userID string, show domain.MediaSearchResult) error {
	if server.library == nil {
		return fmt.Errorf("library is not configured")
	}
	mediaID := fmt.Sprintf("tv:%d", show.TMDBID)
	entries, err := server.library.List(ctx, userID)
	if err != nil {
		return err
	}
	found := false
	for _, entry := range entries {
		if entry.Item.MediaID == mediaID {
			found = true
			if entry.Item.Status != domain.WatchingStatus {
				if _, err := server.library.Save(ctx, userID, mediaID, domain.WatchingStatus, entry.Item.Rating); err != nil {
					return err
				}
			}
			break
		}
	}
	if !found {
		if _, err := server.saveMedia(ctx, userID, show, domain.WatchingStatus); err != nil {
			return err
		}
		return nil
	}
	provider, ok := server.metadata.(domain.TVShowMetadataProvider)
	if !ok {
		return fmt.Errorf("TV show metadata is not configured")
	}
	if err := server.library.ImportShow(ctx, show.TMDBID, provider); err != nil {
		return fmt.Errorf("could not import show episode catalog: %w", err)
	}
	return nil
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func intArg(args map[string]any, key string) int {
	switch value := args[key].(type) {
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func dateArg(args map[string]any, key string) (time.Time, error) {
	value := stringArg(args, key)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD", key)
	}
	return parsed, nil
}

func timestampArg(args map[string]any, key string) (time.Time, error) {
	value := stringArg(args, key)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be an RFC3339 timestamp", key)
	}
	return parsed, nil
}

func ratingArg(args map[string]any) (*int, error) {
	if args["rating"] == nil {
		return nil, nil
	}
	value := intArg(args, "rating")
	if value < 1 || value > 5 {
		return nil, fmt.Errorf("rating must be from 1 to 5")
	}
	return &value, nil
}

func mediaArg(args map[string]any) (domain.MediaSearchResult, error) {
	var item domain.MediaSearchResult
	data, err := json.Marshal(args["media"])
	if err != nil || json.Unmarshal(data, &item) != nil {
		return item, fmt.Errorf("media must be a result returned by search_media")
	}
	if item.TMDBID <= 0 || item.Title == "" || (item.Type != domain.MovieMediaType && item.Type != domain.TVMediaType) {
		return item, fmt.Errorf("media must include a valid TMDB ID, type, and title")
	}
	return item, nil
}
