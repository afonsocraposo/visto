package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/oauth"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestChatGPTOAuthFlow_GivenReadOnlyConsent_WhenConnecting_ThenReadsOwnHistoryAndDeniesWrites(t *testing.T) {
	store, err := sqlite.Open(context.Background(), t.TempDir()+"/mcp-oauth.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	authService := auth.NewService(store)
	if _, err := authService.Bootstrap(context.Background(), "mcp-user@example.com", "MCP User", "a-strong-test-password"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	server := New(authService, oauth.NewService(store), "https://visto.example", nil, library.NewService(store), tracking.NewService(store), watch.NewService(store))

	registration := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"ChatGPT","redirect_uris":["https://chatgpt.com/connector_platform_oauth_redirect"],"grant_types":["authorization_code","refresh_token"],"response_types":["code"],"token_endpoint_auth_method":"none"}`))
	registration.Header.Set("Content-Type", "application/json")
	registrationResponse := httptest.NewRecorder()
	server.ServeHTTP(registrationResponse, registration)
	if registrationResponse.Code != http.StatusCreated {
		t.Fatalf("registration status = %d, body: %s", registrationResponse.Code, registrationResponse.Body.String())
	}
	var client struct {
		ID string `json:"client_id"`
	}
	if err := json.Unmarshal(registrationResponse.Body.Bytes(), &client); err != nil {
		t.Fatalf("decode client: %v", err)
	}

	verifier := strings.Repeat("x", 43)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	redirectURI := "https://chatgpt.com/connector_platform_oauth_redirect"
	resource := "https://visto.example/mcp"
	params := url.Values{"response_type": {"code"}, "client_id": {client.ID}, "redirect_uri": {redirectURI}, "state": {"test-state"}, "scope": {oauth.ReadScope}, "resource": {resource}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	getAuth := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+params.Encode(), nil)
	getAuthResponse := httptest.NewRecorder()
	server.ServeHTTP(getAuthResponse, getAuth)
	if getAuthResponse.Code != http.StatusOK {
		t.Fatalf("authorize form status = %d", getAuthResponse.Code)
	}
	csrfMatch := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(getAuthResponse.Body.String())
	if len(csrfMatch) != 2 {
		t.Fatal("authorization form did not include a CSRF token")
	}
	csrfCookie := getAuthResponse.Result().Cookies()[0]
	form := url.Values{"csrf": {csrfMatch[1]}, "response_type": {"code"}, "client_id": {client.ID}, "redirect_uri": {redirectURI}, "state": {"test-state"}, "resource": {resource}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}, "scope": {oauth.ReadScope}, "email": {"mcp-user@example.com"}, "password": {"a-strong-test-password"}, "consent": {"allow"}}
	postAuth := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	postAuth.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postAuth.AddCookie(csrfCookie)
	postAuthResponse := httptest.NewRecorder()
	server.ServeHTTP(postAuthResponse, postAuth)
	if postAuthResponse.Code != http.StatusFound {
		t.Fatalf("authorize status = %d, body: %s", postAuthResponse.Code, postAuthResponse.Body.String())
	}
	location, err := url.Parse(postAuthResponse.Header().Get("Location"))
	if err != nil || location.Query().Get("state") != "test-state" || location.Query().Get("iss") != "https://visto.example" {
		t.Fatalf("invalid authorization redirect: %v", err)
	}
	code := location.Query().Get("code")
	if code == "" {
		t.Fatalf("authorization code missing: %s", location)
	}

	tokenForm := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {client.ID}, "redirect_uri": {redirectURI}, "code_verifier": {verifier}, "resource": {resource}}
	tokenRequest := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResponse := httptest.NewRecorder()
	server.ServeHTTP(tokenResponse, tokenRequest)
	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("token status = %d, body: %s", tokenResponse.Code, tokenResponse.Body.String())
	}
	var tokens oauth.IssuedTokens
	if err := json.Unmarshal(tokenResponse.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("decode tokens: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken != "" {
		t.Fatalf("unexpected read-only token response: %+v", tokens)
	}

	readBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_watch_history","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"ChatGPT","version":"test"},"io.modelcontextprotocol/clientCapabilities":{}}}}`
	readResponse := mcpRequest(server, tokens.AccessToken, "tools/call", "get_watch_history", readBody)
	if readResponse.Code != http.StatusOK || !strings.Contains(readResponse.Body.String(), `"structuredContent":[]`) {
		t.Fatalf("read tool call failed: %d %s", readResponse.Code, readResponse.Body.String())
	}
	writeBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"mark_movie_watched","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"ChatGPT","version":"test"},"io.modelcontextprotocol/clientCapabilities":{}}}}`
	writeResponse := mcpRequest(server, tokens.AccessToken, "tools/call", "mark_movie_watched", writeBody)
	if writeResponse.Code != http.StatusOK || !strings.Contains(writeResponse.Body.String(), "insufficient_scope") {
		t.Fatalf("write tool should require write scope: %d %s", writeResponse.Code, writeResponse.Body.String())
	}
}

func mcpRequest(handler http.Handler, token, method, name, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("MCP-Protocol-Version", currentProtocolVersion)
	request.Header.Set("Mcp-Method", method)
	request.Header.Set("Mcp-Name", name)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
