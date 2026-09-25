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

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/oauth"
	"github.com/afonsocosta/visto/internal/domain"
)

type oauthPageData struct {
	ClientName string
	UserEmail  string
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
		user, err := server.sessionUser(r)
		if err != nil {
			server.redirectToSignIn(w, r.URL.RequestURI())
			return
		}
		server.renderAuthorizationForm(w, r, client.Name, user.Email, request, "")
		return
	}
	csrfCookie, err := r.Cookie("visto_oauth_csrf")
	formCSRF := r.Form.Get("csrf")
	if err != nil || formCSRF == "" || subtle.ConstantTimeCompare([]byte(csrfCookie.Value), []byte(formCSRF)) != 1 {
		user, authErr := server.sessionUser(r)
		if authErr != nil {
			server.redirectToSignIn(w, authorizationRequestURI(request))
			return
		}
		server.renderAuthorizationForm(w, r, client.Name, user.Email, request, "This authorization request expired. Please try again.")
		return
	}
	if r.Form.Get("consent") != "allow" {
		server.redirectOAuthError(w, request, "access_denied", "The user cancelled authorization")
		return
	}
	user, err := server.sessionUser(r)
	if err != nil {
		server.redirectToSignIn(w, authorizationRequestURI(request))
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

func (server *Server) sessionUser(r *http.Request) (domain.User, error) {
	if server.credentials == nil {
		return domain.User{}, auth.ErrInvalidCredentials
	}
	cookie, err := r.Cookie("visto_session")
	if err != nil {
		return domain.User{}, err
	}
	return server.credentials.Authenticate(r.Context(), cookie.Value)
}

func (server *Server) redirectToSignIn(w http.ResponseWriter, returnTo string) {
	login := url.URL{Path: "/"}
	query := login.Query()
	query.Set("oauth_return", returnTo)
	login.RawQuery = query.Encode()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Location", login.String())
	w.WriteHeader(http.StatusFound)
}

func authorizationRequestURI(request oauth.AuthorizeRequest) string {
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {request.ClientID},
		"redirect_uri":          {request.RedirectURI},
		"state":                 {request.State},
		"scope":                 {strings.Join(request.Scopes, " ")},
		"resource":              {request.Resource},
		"code_challenge":        {request.CodeChallenge},
		"code_challenge_method": {"S256"},
	}
	return "/oauth/authorize?" + query.Encode()
}

func (server *Server) renderAuthorizationForm(w http.ResponseWriter, r *http.Request, clientName, userEmail string, request oauth.AuthorizeRequest, message string) {
	csrf := make([]byte, 32)
	if _, err := rand.Read(csrf); err != nil {
		http.Error(w, "could not start authorization", http.StatusInternalServerError)
		return
	}
	csrfValue := base64.RawURLEncoding.EncodeToString(csrf)
	secure := server.proxies.IsHTTPS(r)
	http.SetCookie(w, &http.Cookie{Name: "visto_oauth_csrf", Value: csrfValue, Path: "/oauth/authorize", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	data := oauthPageData{ClientName: clientName, UserEmail: userEmail, CSRF: csrfValue, Error: message, Hidden: []oauthHidden{
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
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = template.Must(template.New("authorize").Parse(oauthLoginPage)).Execute(w, data)
}

func parseAuthorizeRequest(params url.Values) oauth.AuthorizeRequest {
	scopes := []string{}
	for _, value := range params["scope"] {
		scopes = append(scopes, strings.Fields(value)...)
	}
	return oauth.AuthorizeRequest{ClientID: params.Get("client_id"), RedirectURI: params.Get("redirect_uri"), State: params.Get("state"), Scopes: scopes, Resource: params.Get("resource"), CodeChallenge: params.Get("code_challenge"), ChallengeMethod: params.Get("code_challenge_method")}
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
