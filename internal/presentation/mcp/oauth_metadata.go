package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/oauth"
)

func (server *Server) mcpResource() string {
	if server.publicURL == "" {
		return ""
	}
	return server.publicURL + "/mcp"
}

func ValidatePublicURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("VISTO_PUBLIC_URL must be an origin with no path, query, or fragment")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && (strings.EqualFold(parsed.Hostname(), "localhost") || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1") {
		return nil
	}
	return fmt.Errorf("VISTO_PUBLIC_URL must use HTTPS (HTTP is allowed only for localhost)")
}

func (server *Server) resourceMetadataURL() string {
	if server.publicURL == "" {
		return ""
	}
	return server.publicURL + "/.well-known/oauth-protected-resource/mcp"
}

func (server *Server) writeOAuthChallenge(w http.ResponseWriter, scope, errorCode string) {
	metadata := server.resourceMetadataURL()
	challenge := `Bearer`
	if metadata != "" {
		challenge = fmt.Sprintf(`Bearer resource_metadata=%q, scope=%q`, metadata, scope)
	}
	if errorCode != "" {
		challenge += fmt.Sprintf(`, error=%q`, errorCode)
	}
	w.Header().Set("WWW-Authenticate", challenge)
}

func (server *Server) protectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if server.publicURL == "" {
		http.Error(w, "VISTO_PUBLIC_URL is required for OAuth", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"resource": server.mcpResource(), "authorization_servers": []string{server.publicURL}, "scopes_supported": []string{oauth.ReadScope, oauth.WriteScope, oauth.OfflineScope}, "resource_documentation": server.publicURL + "/docs/mcp"})
}

func (server *Server) authorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if server.publicURL == "" {
		http.Error(w, "VISTO_PUBLIC_URL is required for OAuth", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer": server.publicURL,
		"authorization_response_iss_parameter_supported": true,
		"authorization_endpoint":                         server.publicURL + "/oauth/authorize",
		"token_endpoint":                                 server.publicURL + "/oauth/token",
		"revocation_endpoint":                            server.publicURL + "/oauth/revoke",
		"registration_endpoint":                          server.publicURL + "/oauth/register",
		"client_id_metadata_document_supported":          false,
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"response_types_supported":                       []string{"code"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"code_challenge_methods_supported":               []string{"S256"},
		"scopes_supported":                               []string{oauth.ReadScope, oauth.WriteScope, oauth.OfflineScope},
	})
}

func (server *Server) registerOAuthClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if server.oauth == nil {
		http.Error(w, "OAuth is not configured", http.StatusServiceUnavailable)
		return
	}
	if !server.rateLimit(w, r, "register", 10, time.Hour) {
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeOAuthError(w, http.StatusUnsupportedMediaType, "invalid_client_metadata", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	defer r.Body.Close()
	var request struct {
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "invalid JSON")
		return
	}
	if !onlyValues(request.GrantTypes, "authorization_code", "refresh_token") || !onlyValues(request.ResponseTypes, "code") || request.TokenEndpointAuthMethod != "" && request.TokenEndpointAuthMethod != "none" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "only public authorization-code clients with PKCE are supported")
		return
	}
	client, err := server.oauth.RegisterClient(r.Context(), request.ClientName, request.RedirectURIs)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri", "client name or redirect URI is invalid")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]any{"client_id": client.ID, "client_name": client.Name, "redirect_uris": client.RedirectURIs, "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}, "token_endpoint_auth_method": "none"})
}
