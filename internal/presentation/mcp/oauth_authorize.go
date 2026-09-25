package mcp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/oauth"
)

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
	user, err := server.credentials.AuthenticateCredentials(r.Context(), r.Form.Get("email"), r.Form.Get("password"))
	if err != nil {
		server.renderAuthorizationForm(w, r, client.Name, request, "Email or password is incorrect.")
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
