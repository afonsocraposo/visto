package httpserver

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/presentation/security"
	"golang.org/x/oauth2"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func (config GoogleOAuthConfig) Enabled() bool {
	if strings.TrimSpace(config.ClientID) == "" || strings.TrimSpace(config.ClientSecret) == "" {
		return false
	}
	redirect, err := url.Parse(strings.TrimSpace(config.RedirectURL))
	if err != nil || redirect.Host == "" || redirect.User != nil || redirect.Fragment != "" || redirect.Path != "/api/v1/auth/google/callback" {
		return false
	}
	return redirect.Scheme == "https" || redirect.Scheme == "http" && (redirect.Hostname() == "localhost" || redirect.Hostname() == "127.0.0.1" || redirect.Hostname() == "::1")
}

func (config GoogleOAuthConfig) oauthConfig() *oauth2.Config {
	return &oauth2.Config{ClientID: config.ClientID, ClientSecret: config.ClientSecret, RedirectURL: config.RedirectURL, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/auth", TokenURL: "https://oauth2.googleapis.com/token"}, Scopes: []string{"openid", "email", "profile"}}
}

func googleLogin(config GoogleOAuthConfig, service *auth.Service, proxies *security.ProxyResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil || !service.GoogleEnabled() {
			http.NotFound(w, r)
			return
		}
		state := make([]byte, 32)
		if _, err := rand.Read(state); err != nil {
			writeError(w, http.StatusInternalServerError, "could not start Google sign-in")
			return
		}
		stateValue := base64.RawURLEncoding.EncodeToString(state)
		http.SetCookie(w, &http.Cookie{Name: "visto_google_state", Value: stateValue, Path: "/api/v1/auth/google/callback", HttpOnly: true, Secure: cookieSecure(r, proxies), SameSite: http.SameSiteLaxMode, MaxAge: 600})
		returnTo := validOAuthReturnTo(r.URL.Query().Get("return_to"))
		returnCookie := &http.Cookie{Name: "visto_google_return", Path: "/api/v1/auth/google/callback", HttpOnly: true, Secure: cookieSecure(r, proxies), SameSite: http.SameSiteLaxMode}
		if returnTo == "" {
			returnCookie.Expires = time.Unix(1, 0)
			returnCookie.MaxAge = -1
		} else {
			returnCookie.Value = returnTo
			returnCookie.MaxAge = 600
		}
		http.SetCookie(w, returnCookie)
		http.Redirect(w, r, config.oauthConfig().AuthCodeURL(stateValue, oauth2.AccessTypeOnline), http.StatusFound)
	}
}

type googleProfile struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func googleCallback(config GoogleOAuthConfig, service *auth.Service, proxies *security.ProxyResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		returnTo := ""
		if cookie, err := r.Cookie("visto_google_return"); err == nil {
			returnTo = validOAuthReturnTo(cookie.Value)
		}
		clearGoogleState(w, r, proxies)
		stateCookie, err := r.Cookie("visto_google_state")
		state := r.URL.Query().Get("state")
		if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 || r.URL.Query().Get("error") != "" {
			googleFailure(w, r, "", returnTo)
			return
		}
		if service == nil || !service.GoogleEnabled() {
			http.NotFound(w, r)
			return
		}
		token, err := config.oauthConfig().Exchange(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			googleFailure(w, r, "", returnTo)
			return
		}
		client := config.oauthConfig().Client(r.Context(), token)
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
		if err != nil {
			googleFailure(w, r, "", returnTo)
			return
		}
		response, err := client.Do(request)
		if err != nil {
			googleFailure(w, r, "", returnTo)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			googleFailure(w, r, "", returnTo)
			return
		}
		var profile googleProfile
		if err := json.NewDecoder(http.MaxBytesReader(w, response.Body, 64<<10)).Decode(&profile); err != nil || profile.Subject == "" || !profile.EmailVerified || profile.Email == "" {
			googleFailure(w, r, "", returnTo)
			return
		}
		if profile.Name == "" {
			profile.Name = profile.Email
		}
		_, sessionToken, expiresAt, err := service.LoginWithGoogle(r.Context(), profile.Subject, profile.Email, profile.Name)
		if err != nil {
			message := ""
			if errors.Is(err, auth.ErrSignupsDisabled) {
				message = "signup_disabled"
			}
			googleFailure(w, r, message, returnTo)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "visto_session", Value: sessionToken, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expiresAt, Secure: cookieSecure(r, proxies)})
		if returnTo == "" {
			returnTo = "/"
		}
		http.Redirect(w, r, returnTo, http.StatusFound)
	}
}

func clearGoogleState(w http.ResponseWriter, r *http.Request, proxies *security.ProxyResolver) {
	http.SetCookie(w, &http.Cookie{Name: "visto_google_state", Value: "", Path: "/api/v1/auth/google/callback", HttpOnly: true, Secure: cookieSecure(r, proxies), SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "visto_google_return", Value: "", Path: "/api/v1/auth/google/callback", HttpOnly: true, Secure: cookieSecure(r, proxies), SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1})
}

func googleFailure(w http.ResponseWriter, r *http.Request, reason, returnTo string) {
	query := url.Values{"auth_error": {"google"}}
	if reason != "" {
		query.Set("reason", reason)
	}
	if returnTo != "" {
		query.Set("oauth_return", returnTo)
	}
	http.Redirect(w, r, "/?"+query.Encode(), http.StatusFound)
}

func validOAuthReturnTo(raw string) string {
	if raw == "" || len(raw) > 3500 || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return ""
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Path != "/oauth/authorize" || parsed.RawQuery == "" || parsed.Fragment != "" {
		return ""
	}
	return parsed.RequestURI()
}
