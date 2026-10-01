package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

type testLibrary struct {
	libraryUseCases
	userID  string
	options library.ListOptions
	request pagination.Request
	page    pagination.Page[library.Entry]
}

func (service *testLibrary) ListPage(_ context.Context, userID string, options library.ListOptions, request pagination.Request) (pagination.Page[library.Entry], error) {
	service.userID, service.options, service.request = userID, options, request
	return service.page, nil
}

func (*testLibrary) List(context.Context, string) ([]library.Entry, error) {
	return nil, errors.New("get_library must not load the whole library")
}

func TestGetLibrary_GivenArguments_WhenCalled_ThenForwardsFiltersAndPagination(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        map[string]any
		wantOptions library.ListOptions
		wantRequest pagination.Request
	}{
		{"defaults", map[string]any{}, library.ListOptions{}, pagination.Request{Limit: 30}},
		{"explicit limit", map[string]any{"limit": 10}, library.ListOptions{}, pagination.Request{Limit: 10}},
		{"JSON limit", map[string]any{"limit": float64(100)}, library.ListOptions{}, pagination.Request{Limit: 100}},
		{"cursor", map[string]any{"cursor": "next"}, library.ListOptions{}, pagination.Request{Limit: 30, Cursor: "next"}},
		{"status", map[string]any{"status": "watching"}, library.ListOptions{Status: "watching"}, pagination.Request{Limit: 30}},
		{"media type", map[string]any{"media_type": "tv"}, library.ListOptions{MediaType: "tv"}, pagination.Request{Limit: 30}},
		{"status and media type", map[string]any{"status": "completed", "media_type": "movie", "limit": 5}, library.ListOptions{Status: "completed", MediaType: "movie"}, pagination.Request{Limit: 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &testLibrary{}
			server := &Server{library: service}
			if _, err := server.callTool(context.Background(), "user-123", "get_library", tc.args); err != nil {
				t.Fatal(err)
			}
			if service.userID != "user-123" || service.options != tc.wantOptions || service.request != tc.wantRequest {
				t.Fatalf("ListPage(%q, %+v, %+v), want (user-123, %+v, %+v)", service.userID, service.options, service.request, tc.wantOptions, tc.wantRequest)
			}
		})
	}
}

func TestGetLibrary_GivenInvalidArguments_WhenCalled_ThenRejectsThem(t *testing.T) {
	for _, args := range []map[string]any{
		{"limit": 0},
		{"limit": 101},
		{"limit": -1},
		{"limit": 2.5},
		{"limit": "10"},
		{"cursor": 7},
		{"status": "finished"},
		{"media_type": "book"},
	} {
		service := &testLibrary{}
		server := &Server{library: service}
		if _, err := server.callTool(context.Background(), "user-123", "get_library", args); err == nil {
			t.Errorf("get_library(%v) succeeded, want error", args)
		}
		if service.userID != "" {
			t.Errorf("get_library(%v) reached ListPage", args)
		}
	}
}

func TestGetLibrary_GivenPage_WhenCalledOverMCP_ThenReturnsItemsCursorAndTotal(t *testing.T) {
	next, total := "cursor-2", 347
	service := &testLibrary{page: pagination.Page[library.Entry]{
		Items:      []library.Entry{{Item: library.Item{MediaID: "movie:1", Status: domain.WatchingStatus}}},
		NextCursor: &next,
		TotalCount: &total,
	}}
	server := &Server{auth: testAuthenticator{user: domain.User{ID: "user-123"}}, library: service}
	body := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_library","arguments":{"status":"watching","limit":1}}}`
	result := callMCPTool(t, server, body)

	var page struct {
		Items      []json.RawMessage `json:"items"`
		NextCursor *string           `json:"next_cursor"`
		TotalCount *int              `json:"total_count"`
	}
	if err := json.Unmarshal([]byte(result), &page); err != nil {
		t.Fatalf("decode page %s: %v", result, err)
	}
	if len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != next || page.TotalCount == nil || *page.TotalCount != total {
		t.Fatalf("page = %s", result)
	}
	if service.userID != "user-123" {
		t.Fatalf("library user = %q, want authenticated user", service.userID)
	}
}

func TestGetLibrary_GivenMoreThanOnePage_WhenFollowingCursors_ThenReturnsEveryEntryOnce(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, t.TempDir()+"/mcp-library.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := auth.NewService(store).Bootstrap(ctx, "pager@example.com", "Pager", "a-strong-test-password")
	if err != nil {
		t.Fatal(err)
	}
	const movies, shows = 65, 4
	for index := 1; index <= movies+shows; index++ {
		mediaType, status := "movie", "completed"
		if index > movies {
			mediaType, status = "tv", "watching"
		}
		mediaID := fmt.Sprintf("%s:%d", mediaType, index)
		// Shared timestamps exercise the cursor's tie-breaking on media ID.
		updated := fmt.Sprintf("2026-01-%02dT00:00:00Z", index%5+1)
		if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES(?,?,?,?,?,?)`, mediaID, mediaType, index, fmt.Sprintf("Title %d", index), updated, updated); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,?,?,?,?)`, user.ID+mediaID, user.ID, mediaID, status, updated, updated); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{library: library.NewService(store)}

	args := map[string]any{"status": "completed", "media_type": "movie"}
	seen := map[string]bool{}
	pages := 0
	for {
		result, err := server.callTool(ctx, user.ID, "get_library", args)
		if err != nil {
			t.Fatal(err)
		}
		page := result.(pagination.Page[library.Entry])
		pages++
		if page.TotalCount == nil || *page.TotalCount != movies {
			t.Fatalf("total_count = %v, want %d", page.TotalCount, movies)
		}
		if len(page.Items) > pagination.DefaultLimit {
			t.Fatalf("page size = %d, want at most %d", len(page.Items), pagination.DefaultLimit)
		}
		for _, entry := range page.Items {
			if seen[entry.Item.MediaID] {
				t.Fatalf("%s returned twice", entry.Item.MediaID)
			}
			seen[entry.Item.MediaID] = true
		}
		if page.NextCursor == nil {
			break
		}
		args["cursor"] = *page.NextCursor
	}
	if len(seen) != movies || pages != 3 {
		t.Fatalf("saw %d entries over %d pages, want %d over 3", len(seen), pages, movies)
	}

	first, err := server.callTool(ctx, user.ID, "get_library", map[string]any{"status": "completed", "limit": 1})
	if err != nil {
		t.Fatal(err)
	}
	cursor := first.(pagination.Page[library.Entry]).NextCursor
	if cursor == nil {
		t.Fatal("expected a next cursor")
	}
	if _, err := server.callTool(ctx, user.ID, "get_library", map[string]any{"status": "watching", "limit": 1, "cursor": *cursor}); !errors.Is(err, pagination.ErrInvalidCursor) {
		t.Fatalf("cursor reused across filters: error = %v, want invalid cursor", err)
	}
}

func callMCPTool(t *testing.T, server *Server, body string) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
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
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	if payload.Result.IsError || len(payload.Result.Content) == 0 {
		t.Fatalf("tool call failed: %s", response.Body.String())
	}
	return payload.Result.Content[0].Text
}

func TestPagedTools_GivenArguments_WhenCalled_ThenForwardPagination(t *testing.T) {
	from, to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	trackingService := &testTracking{}
	server := &Server{tracking: trackingService, watch: &testWatch{}}
	if _, err := server.callTool(context.Background(), "user-123", "get_watch_history", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if trackingService.userID != "user-123" || trackingService.request != (pagination.Request{Limit: 30}) {
		t.Fatalf("history user=%q request=%+v", trackingService.userID, trackingService.request)
	}
	if _, err := server.callTool(context.Background(), "user-123", "get_watch_history", map[string]any{"limit": 100, "cursor": "next"}); err != nil {
		t.Fatal(err)
	}
	if trackingService.request != (pagination.Request{Limit: 100, Cursor: "next"}) {
		t.Fatalf("history request = %+v", trackingService.request)
	}

	for _, tc := range []struct {
		name       string
		args       map[string]any
		wantParent string
	}{
		{"whole show", map[string]any{"show_id": "tv:42", "limit": 10}, "tv:42"},
		{"one season", map[string]any{"show_id": "tv:42", "season_number": 3, "limit": 10}, "tv:42:season:3"},
		{"specials", map[string]any{"show_id": "tv:42", "season_number": 0, "limit": 10}, "tv:42:season:0"},
	} {
		watchService := &testWatch{}
		server := &Server{watch: watchService}
		if _, err := server.callTool(context.Background(), "user-123", "get_show_episodes", tc.args); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if watchService.parentID != tc.wantParent || watchService.request != (pagination.Request{Limit: 10}) {
			t.Fatalf("%s: parent=%q request=%+v", tc.name, watchService.parentID, watchService.request)
		}
	}

	watchService := &testWatch{}
	server = &Server{watch: watchService}
	if _, err := server.callTool(context.Background(), "user-123", "get_upcoming_episodes", map[string]any{"from": "2026-10-01", "to": "2026-12-01", "cursor": "next"}); err != nil {
		t.Fatal(err)
	}
	if !watchService.from.Equal(from) || !watchService.to.Equal(to) || watchService.request != (pagination.Request{Limit: 30, Cursor: "next"}) {
		t.Fatalf("calendar from=%v to=%v request=%+v", watchService.from, watchService.to, watchService.request)
	}
}

func TestPagedTools_GivenInvalidLimit_WhenCalled_ThenRejectIt(t *testing.T) {
	server := &Server{tracking: &testTracking{}, watch: &testWatch{}}
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"get_watch_history", map[string]any{"limit": 500}},
		{"get_watch_history", map[string]any{"limit": 0}},
		{"get_show_episodes", map[string]any{"show_id": "tv:42", "limit": 101}},
		{"get_upcoming_episodes", map[string]any{"limit": -1}},
	} {
		if _, err := server.callTool(context.Background(), "user-123", tc.tool, tc.args); !errors.Is(err, pagination.ErrInvalidLimit) {
			t.Errorf("%s(%v) error = %v, want invalid limit", tc.tool, tc.args, err)
		}
	}
}

func TestGetShowEpisodes_GivenMissingSeason_WhenCalled_ThenReturnsEmptyPage(t *testing.T) {
	server := &Server{watch: &testWatch{missingSeason: true}}
	result, err := server.callTool(context.Background(), "user-123", "get_show_episodes", map[string]any{"show_id": "tv:42", "season_number": 9})
	if err != nil {
		t.Fatal(err)
	}
	page := result.(pagination.Page[watch.ShowEpisode])
	if page.Items == nil || len(page.Items) != 0 || page.NextCursor != nil {
		t.Fatalf("page = %+v, want empty", page)
	}
}

func TestPagedTools_GivenSQLiteStore_WhenFollowingCursors_ThenReturnEveryItemOnce(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, t.TempDir()+"/mcp-paging.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := auth.NewService(store).Bootstrap(ctx, "pager@example.com", "Pager", "a-strong-test-password")
	if err != nil {
		t.Fatal(err)
	}
	const seasons, episodesPerSeason = 3, 25
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := store.DB.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Long Show',?,?)`, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
	exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','watching',?,?)`, user.ID+"tv:42", user.ID, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
	exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:7','tv',7,'Untracked Show',?,?)`, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
	for season := 1; season <= seasons; season++ {
		seasonID := fmt.Sprintf("tv:42:season:%d", season)
		exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES(?,'tv:42',?,'Season')`, seasonID, season)
		for episode := 1; episode <= episodesPerSeason; episode++ {
			episodeID := fmt.Sprintf("tv:42:episode:%d%02d", season, episode)
			exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES(?,'tv:42',?,?,?,'Episode')`, episodeID, seasonID, season, episode)
			// Every play shares one of a few timestamps to exercise tie-breaking on play ID.
			watchedAt := fmt.Sprintf("2026-02-%02dT00:00:00Z", episode%3+1)
			exec(`INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES(?,?,?,'web',?)`, user.ID, episodeID, watchedAt, watchedAt)
		}
	}
	server := &Server{tracking: tracking.NewService(store), watch: watch.NewService(store)}

	plays := collectPages(t, server, user.ID, "get_watch_history", map[string]any{}, func(entry tracking.HistoryEntry) string { return entry.Play.ID })
	if len(plays) != seasons*episodesPerSeason {
		t.Fatalf("history returned %d plays, want %d", len(plays), seasons*episodesPerSeason)
	}
	episodes := collectPages(t, server, user.ID, "get_show_episodes", map[string]any{"show_id": "tv:42"}, func(entry watch.ShowEpisode) string { return entry.Episode.ID })
	if len(episodes) != seasons*episodesPerSeason || episodes[0] != "tv:42:episode:101" || episodes[len(episodes)-1] != "tv:42:episode:325" {
		t.Fatalf("show episodes = %d, first %q, last %q", len(episodes), episodes[0], episodes[len(episodes)-1])
	}
	seasonTwo := collectPages(t, server, user.ID, "get_show_episodes", map[string]any{"show_id": "tv:42", "season_number": 2, "limit": 10}, func(entry watch.ShowEpisode) string { return entry.Episode.ID })
	if len(seasonTwo) != episodesPerSeason || seasonTwo[0] != "tv:42:episode:201" {
		t.Fatalf("season 2 episodes = %v", seasonTwo)
	}
	if missing := collectPages(t, server, user.ID, "get_show_episodes", map[string]any{"show_id": "tv:42", "season_number": 9}, func(entry watch.ShowEpisode) string { return entry.Episode.ID }); len(missing) != 0 {
		t.Fatalf("missing season episodes = %v", missing)
	}
	if _, err := server.callTool(ctx, user.ID, "get_show_episodes", map[string]any{"show_id": "tv:7", "season_number": 1}); !errors.Is(err, watch.ErrShowNotFound) {
		t.Fatalf("untracked show error = %v, want show not found", err)
	}
}

// collectPages follows next_cursor until the last page and returns each
// item's key, failing on duplicates or oversized pages.
func collectPages[T any](t *testing.T, server *Server, userID, tool string, args map[string]any, key func(T) string) []string {
	t.Helper()
	keys := []string{}
	seen := map[string]bool{}
	for {
		result, err := server.callTool(context.Background(), userID, tool, args)
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		page := result.(pagination.Page[T])
		if len(page.Items) > pagination.DefaultLimit {
			t.Fatalf("%s page size = %d, want at most %d", tool, len(page.Items), pagination.DefaultLimit)
		}
		for _, item := range page.Items {
			if seen[key(item)] {
				t.Fatalf("%s returned %s twice", tool, key(item))
			}
			seen[key(item)] = true
			keys = append(keys, key(item))
		}
		if page.NextCursor == nil {
			return keys
		}
		args["cursor"] = *page.NextCursor
	}
}
