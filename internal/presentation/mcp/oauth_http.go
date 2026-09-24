package mcp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/oauth"
	"github.com/afonsocosta/visto/internal/presentation/security"
)

const oauthLoginPage = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Connect Visto</title>
<style>body{font:16px system-ui;background:#11141b;color:#f4f4f5;display:grid;place-items:center;min-height:100vh;margin:0}.panel{width:min(440px,calc(100% - 40px));padding:28px;border:1px solid #3b3e46;border-radius:18px;background:#1b1e26}h1{margin:0 0 8px}p{color:#b7bac3;line-height:1.5}label{display:block;margin:16px 0 6px}input[type=text],input[type=password]{box-sizing:border-box;width:100%;padding:12px;border-radius:10px;border:1px solid #4a4d55;background:#11141b;color:#fff}input[type=checkbox]{accent-color:#f5b52e}button{margin-top:22px;padding:12px 18px;border:0;border-radius:10px;background:#f5b52e;color:#1b1e26;font-weight:700;cursor:pointer}.scope{padding:10px 0;border-top:1px solid #393c44}.error{color:#ff9898}</style>
<main class="panel"><h1>Connect Visto</h1><p><strong>{{.ClientName}}</strong> wants permission to:</p>{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
<form method="post" action="/oauth/authorize"><input type="hidden" name="csrf" value="{{.CSRF}}">{{range .Hidden}}<input type="hidden" name="{{.Name}}" value="{{.Value}}">{{end}}
<div class="scope"><label><input type="checkbox" name="scope" value="read" {{if .Read}}checked{{end}}> Read my Visto library, progress, and watch history</label></div>
<div class="scope"><label><input type="checkbox" name="scope" value="write" {{if .Write}}checked{{end}}> Add titles, update lists, mark media watched, and rate it</label></div>
<div class="scope"><label><input type="checkbox" name="scope" value="offline_access" {{if .Offline}}checked{{end}}> Stay connected between visits</label></div>
<label for="username">Visto username</label><input id="username" name="username" type="text" autocomplete="username" required>
<label for="password">Visto password</label><input id="password" name="password" type="password" autocomplete="current-password" required>
<button name="consent" value="allow">Authorize ChatGPT</button><button formnovalidate name="consent" value="deny" style="background:#383b43;color:#fff;margin-left:8px">Cancel</button></form></main></html>`

type oauthPageData struct {
	ClientName string
	CSRF       string
	Error      string
	Read       bool
	Write      bool
	Offline    bool
	Hidden     []oauthHidden
}

type oauthHidden struct{ Name, Value string }

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

func (server *Server) authorizeOAuthClient(w http.ResponseWriter, r *http.Request) {
	if server.oauth == nil || server.auth == nil {
		http.Error(w, "OAuth is not configured", http.StatusServiceUnavailable)
		return
	}
	if server.publicURL == "" {
		http.Error(w, "VISTO_PUBLIC_URL is required for OAuth", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	params := r.URL.Query()
	if r.Method == http.MethodPost {
		if !server.rateLimit(w, r, "authorize", 12, 15*time.Minute) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		defer r.Body.Close()
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid authorization form", http.StatusBadRequest)
			return
		}
		params = r.Form
	}
	request := parseAuthorizeRequest(params)
	client, err := server.oauth.Client(r.Context(), request.ClientID)
	if err != nil || !containsValue(client.RedirectURIs, request.RedirectURI) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "client or redirect URI is invalid")
		return
	}
	if err := validateAuthorizeRequest(request, server.mcpResource()); err != nil {
		server.redirectOAuthError(w, request, "invalid_request", err.Error())
		return
	}
	if r.Method == http.MethodGet {
		server.renderAuthorizationForm(w, r, client.Name, request, "")
		return
	}
	csrfCookie, err := r.Cookie("visto_oauth_csrf")
	formCSRF := r.Form.Get("csrf")
	if err != nil || formCSRF == "" || subtle.ConstantTimeCompare([]byte(csrfCookie.Value), []byte(formCSRF)) != 1 {
		server.renderAuthorizationForm(w, r, client.Name, request, "This authorization request expired. Please try again.")
		return
	}
	if r.Form.Get("consent") != "allow" {
		server.redirectOAuthError(w, request, "access_denied", "The user cancelled authorization")
		return
	}
	if server.credentials == nil {
		http.Error(w, "authentication is not configured", http.StatusServiceUnavailable)
		return
	}
	user, err := server.credentials.AuthenticateCredentials(r.Context(), r.Form.Get("username"), r.Form.Get("password"))
	if err != nil {
		server.renderAuthorizationForm(w, r, client.Name, request, "Username or password is incorrect.")
		return
	}
	request.Scopes = r.Form["scope"]
	code, err := server.oauth.CreateAuthorizationCode(r.Context(), user.ID, request, server.mcpResource())
	if err != nil {
		server.redirectOAuthError(w, request, "invalid_scope", "Choose at least one valid Visto permission")
		return
	}
	redirect, _ := url.Parse(request.RedirectURI)
	query := redirect.Query()
	query.Set("code", code)
	if request.State != "" {
		query.Set("state", request.State)
	}
	query.Set("iss", server.publicURL)
	redirect.RawQuery = query.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (server *Server) renderAuthorizationForm(w http.ResponseWriter, r *http.Request, clientName string, request oauth.AuthorizeRequest, message string) {
	csrf := make([]byte, 32)
	if _, err := rand.Read(csrf); err != nil {
		http.Error(w, "could not start authorization", http.StatusInternalServerError)
		return
	}
	csrfValue := base64.RawURLEncoding.EncodeToString(csrf)
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{Name: "visto_oauth_csrf", Value: csrfValue, Path: "/oauth/authorize", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	data := oauthPageData{ClientName: clientName, CSRF: csrfValue, Error: message, Hidden: []oauthHidden{
		{Name: "response_type", Value: "code"}, {Name: "client_id", Value: request.ClientID}, {Name: "redirect_uri", Value: request.RedirectURI},
		{Name: "state", Value: request.State}, {Name: "resource", Value: request.Resource}, {Name: "code_challenge", Value: request.CodeChallenge}, {Name: "code_challenge_method", Value: "S256"},
	}}
	requested := map[string]bool{}
	for _, scope := range request.Scopes {
		requested[scope] = true
	}
	data.Read, data.Write, data.Offline = requested[oauth.ReadScope], requested[oauth.WriteScope], requested[oauth.OfflineScope]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = template.Must(template.New("authorize").Parse(oauthLoginPage)).Execute(w, data)
}

func (server *Server) issueOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if server.oauth == nil || server.publicURL == "" {
		http.Error(w, "OAuth is not configured", http.StatusServiceUnavailable)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		writeOAuthError(w, http.StatusUnsupportedMediaType, "invalid_request", "Content-Type must be application/x-www-form-urlencoded")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	defer r.Body.Close()
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid form data")
		return
	}
	var tokens oauth.IssuedTokens
	var err error
	resource := r.Form.Get("resource")
	if resource != server.mcpResource() {
		writeOAuthError(w, http.StatusBadRequest, "invalid_target", "resource does not match this Visto instance")
		return
	}
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		tokens, err = server.oauth.ExchangeCode(r.Context(), r.Form.Get("code"), r.Form.Get("client_id"), r.Form.Get("redirect_uri"), r.Form.Get("code_verifier"), resource)
	case "refresh_token":
		tokens, err = server.oauth.Refresh(r.Context(), r.Form.Get("refresh_token"), r.Form.Get("client_id"), resource)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant type is not supported")
		return
	}
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization grant is invalid, expired, or already used")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, tokens)
}

func (server *Server) revokeOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if server.oauth == nil {
		http.Error(w, "OAuth is not configured", http.StatusServiceUnavailable)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		writeOAuthError(w, http.StatusUnsupportedMediaType, "invalid_request", "Content-Type must be application/x-www-form-urlencoded")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	defer r.Body.Close()
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "invalid form data")
		return
	}
	if err := server.oauth.Revoke(r.Context(), r.Form.Get("token")); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "token revocation failed")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func parseAuthorizeRequest(params url.Values) oauth.AuthorizeRequest {
	return oauth.AuthorizeRequest{ClientID: params.Get("client_id"), RedirectURI: params.Get("redirect_uri"), State: params.Get("state"), Scopes: strings.Fields(params.Get("scope")), Resource: params.Get("resource"), CodeChallenge: params.Get("code_challenge"), ChallengeMethod: params.Get("code_challenge_method")}
}

func validateAuthorizeRequest(request oauth.AuthorizeRequest, resource string) error {
	if request.Resource != resource || request.State == "" || len(request.State) > 512 || request.ChallengeMethod != "S256" || request.CodeChallenge == "" {
		return oauth.ErrInvalidRequest
	}
	hasPermission := false
	for _, scope := range request.Scopes {
		switch scope {
		case oauth.ReadScope, oauth.WriteScope:
			hasPermission = true
		case oauth.OfflineScope:
		default:
			return oauth.ErrInvalidRequest
		}
	}
	if len(request.Scopes) > 0 && !hasPermission {
		return oauth.ErrInvalidRequest
	}
	return nil
}

func (server *Server) redirectOAuthError(w http.ResponseWriter, request oauth.AuthorizeRequest, code, description string) {
	redirect, err := url.Parse(request.RedirectURI)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, code, description)
		return
	}
	query := redirect.Query()
	query.Set("error", code)
	query.Set("error_description", description)
	if request.State != "" {
		query.Set("state", request.State)
	}
	query.Set("iss", server.publicURL)
	redirect.RawQuery = query.Encode()
	w.Header().Set("Location", redirect.String())
	w.WriteHeader(http.StatusFound)
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}

func (server *Server) rateLimit(w http.ResponseWriter, r *http.Request, bucket string, limit int, window time.Duration) bool {
	if server.rateLimiter == nil {
		server.rateLimiter = security.NewRateLimiter(10_000)
	}
	allowed, retryAfter := server.rateLimiter.Allow(bucket+":"+server.proxies.ClientIP(r), limit, window)
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		http.Error(w, "Too many OAuth requests", http.StatusTooManyRequests)
		return false
	}
	return true
}

func onlyValues(values []string, allowed ...string) bool {
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if !containsValue(allowed, value) {
			return false
		}
	}
	return true
}

func containsValue(values []string, sought string) bool {
	for _, value := range values {
		if value == sought {
			return true
		}
	}
	return false
}
