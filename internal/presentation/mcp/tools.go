package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/oauth"
	"github.com/afonsocosta/visto/internal/domain"
)

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
