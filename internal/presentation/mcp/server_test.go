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
	"github.com/afonsocosta/visto/internal/domain"
)

type testAuthenticator struct{ user domain.User }

func (authenticator testAuthenticator) AuthenticatePersonalToken(_ context.Context, token string) (domain.User, error) {
	if token != "valid" {
		return domain.User{}, auth.ErrInvalidCredentials
	}
	return authenticator.user, nil
}

type testTracking struct{ userID string }

func (service *testTracking) History(_ context.Context, userID string, _ int) ([]tracking.HistoryEntry, error) {
	service.userID = userID
	return []tracking.HistoryEntry{}, nil
}

func (*testTracking) Record(context.Context, string, *string, *string, time.Time, string) (tracking.Play, error) {
	return tracking.Play{}, nil
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
	if len(payload.Result.Tools) != 10 {
		t.Fatalf("tool count = %d, want 10", len(payload.Result.Tools))
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

func TestMCPHandler_GivenModernDiscoveryRequest_WhenCalled_ThenAdvertisesCurrentAndLegacyVersions(t *testing.T) {
	server := &Server{auth: testAuthenticator{user: domain.User{ID: "user-123"}}}
	body := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("MCP-Protocol-Version", currentProtocolVersion)
	request.Header.Set("Mcp-Method", "server/discover")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), currentProtocolVersion) {
		t.Fatalf("discovery response does not advertise %s: %s", currentProtocolVersion, response.Body.String())
	}
}
