package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
)

type testAuthenticator struct{ user domain.User }

func (authenticator testAuthenticator) AuthenticatePersonalToken(_ context.Context, token string) (domain.User, error) {
	if token != "valid" {
		return domain.User{}, auth.ErrInvalidCredentials
	}
	return authenticator.user, nil
}

type testTracking struct {
	userID, showID  string
	season, episode int
}

func (service *testTracking) History(_ context.Context, userID string, _ int) ([]tracking.HistoryEntry, error) {
	service.userID = userID
	return []tracking.HistoryEntry{}, nil
}

func (*testTracking) Record(context.Context, string, *string, *string, time.Time, string) (tracking.Play, error) {
	return tracking.Play{}, nil
}

func (service *testTracking) MarkEpisodesThrough(_ context.Context, userID, showID string, season, episode int, _ time.Time, _ string) (int, error) {
	service.userID, service.showID, service.season, service.episode = userID, showID, season, episode
	return 2, nil
}

func (*testTracking) MarkSeasonWatched(context.Context, string, string, int, time.Time, string) (int, error) {
	return 0, nil
}

func (*testTracking) MarkSelectedEpisodes(context.Context, string, string, []string, time.Time, string) (int, error) {
	return 0, nil
}

func (*testTracking) RemoveEpisodes(context.Context, string, []string) error { return nil }
func (*testTracking) RemoveMediaAndHistory(_ context.Context, _ string, mediaID string) (tracking.RemovedMedia, error) {
	return tracking.RemovedMedia{MediaID: mediaID}, nil
}

type testWatch struct{ episodes []watch.ShowEpisode }

func (service *testWatch) Episodes(context.Context, string, string) ([]watch.ShowEpisode, error) {
	return service.episodes, nil
}
func (*testWatch) Continue(context.Context, string) ([]watch.ContinueEntry, error) {
	return nil, nil
}
func (*testWatch) Calendar(context.Context, string, time.Time, time.Time) ([]watch.CalendarEntry, error) {
	return nil, nil
}
func (*testWatch) ShowProgress(context.Context, string, string) (watch.Progress, error) {
	return watch.Progress{WatchedEpisodes: 3, MissingPriorEpisodes: 0}, nil
}

func (*testTracking) RateEpisode(context.Context, string, string, *int) (tracking.EpisodeRating, error) {
	return tracking.EpisodeRating{}, nil
}

func TestMCPHandler_GivenValidPersonalToken_WhenToolsListIsRequested_ThenReturnsVistoTools(t *testing.T) {
	server := &Server{auth: testAuthenticator{user: domain.User{ID: "user-123"}}}
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Result struct {
			Tools []toolDefinition `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	if len(payload.Result.Tools) != 17 {
		t.Fatalf("tool count = %d, want 17", len(payload.Result.Tools))
	}
	for _, tool := range payload.Result.Tools {
		if len(tool.SecuritySchemes) != 1 {
			t.Errorf("%s security schemes = %v, want one root-level scheme", tool.Name, tool.SecuritySchemes)
		}
		if required, present := tool.InputSchema["required"]; present && required == nil {
			t.Errorf("%s has null required in its input schema", tool.Name)
		}
		if tool.Name == "remove_media" {
			if tool.Annotations["destructiveHint"] != true || requiredScope(tool.Name) != "write" {
				t.Errorf("remove_media is missing destructive annotation or write scope")
			}
		}
	}
}

func TestMCPHandler_GivenMissingToken_WhenCalled_ThenRequiresAuthentication(t *testing.T) {
	server := &Server{auth: testAuthenticator{user: domain.User{ID: "user-123"}}}
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if response.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("expected WWW-Authenticate header")
	}
}

func TestMCPHandler_GivenHistoryToolCall_WhenAuthenticated_ThenUsesAuthenticatedUser(t *testing.T) {
	trackingService := &testTracking{}
	server := &Server{auth: testAuthenticator{user: domain.User{ID: "user-123"}}, tracking: trackingService}
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_watch_history","arguments":{"limit":10}}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	if trackingService.userID != "user-123" {
		t.Fatalf("history user = %q, want authenticated user", trackingService.userID)
	}
}

func TestMCPTools_GivenTrackedShow_WhenListingAndMarkingThrough_ThenReturnProgress(t *testing.T) {
	trackingService := &testTracking{}
	watchService := &testWatch{episodes: []watch.ShowEpisode{
		{Episode: domain.Episode{ID: "tv:42:episode:1", ShowID: "tv:42", SeasonNumber: 1, EpisodeNumber: 1}},
		{Episode: domain.Episode{ID: "tv:42:episode:2", ShowID: "tv:42", SeasonNumber: 1, EpisodeNumber: 2}},
	}}
	server := &Server{tracking: trackingService, watch: watchService}
	listed, err := server.callTool(context.Background(), "user-123", "get_show_episodes", map[string]any{"show_id": "tv:42", "season_number": 1})
	if err != nil || len(listed.([]watch.ShowEpisode)) != 2 {
		t.Fatalf("listed=%v error=%v", listed, err)
	}
	result, err := server.callTool(context.Background(), "user-123", "mark_episodes_through", map[string]any{"show_id": "tv:42", "season_number": 1, "episode_number": 2})
	if err != nil {
		t.Fatal(err)
	}
	response := result.(map[string]any)
	if response["marked_count"] != 2 || trackingService.userID != "user-123" || trackingService.showID != "tv:42" || trackingService.season != 1 || trackingService.episode != 2 {
		t.Fatalf("result=%v tracking=%+v", response, trackingService)
	}
}

func TestMCPHandler_GivenInitializeRequest_WhenCalled_ThenReturnsVistoServerInfo(t *testing.T) {
	server := &Server{auth: testAuthenticator{user: domain.User{ID: "user-123"}}}
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"name":"visto"`) {
		t.Fatalf("initialize response does not identify Visto: %s", response.Body.String())
	}
}
