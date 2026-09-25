package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

func TestGoogleOAuthConfig_GivenPartialSettings_WhenChecked_ThenItStaysDisabled(t *testing.T) {
	for _, config := range []httpserver.GoogleOAuthConfig{
		{},
		{ClientID: "id", ClientSecret: "secret"},
		{ClientID: "id", RedirectURL: "https://visto.example/api/v1/auth/google/callback"},
	} {
		if config.Enabled() {
			t.Fatalf("partial config should be disabled: %+v", config)
		}
	}
	config := httpserver.GoogleOAuthConfig{ClientID: "id", ClientSecret: "secret", RedirectURL: "https://visto.example/api/v1/auth/google/callback"}
	if !config.Enabled() {
		t.Fatal("complete Google OAuth config should be enabled")
	}
	config.RedirectURL = "http://visto.example/api/v1/auth/google/callback"
	if config.Enabled() {
		t.Fatal("plain HTTP redirect must not be enabled outside localhost")
	}
}

func TestGoogleLogin_GivenConfiguredProvider_WhenStarted_ThenItRedirectsAndSetsSingleUseState(t *testing.T) {
	service := auth.NewService(&accountHTTPRepository{}, auth.Config{GoogleEnabled: true})
	config := httpserver.GoogleOAuthConfig{ClientID: "client-id", ClientSecret: "client-secret", RedirectURL: "https://visto.example/api/v1/auth/google/callback"}
	handler := httpserver.New(service, nil, "", nil, nil, nil, nil, nil, nil).WithGoogleOAuth(config).Handler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/auth/google", nil))
	if response.Code != http.StatusFound || !strings.HasPrefix(response.Header().Get("Location"), "https://accounts.google.com/") {
		t.Fatalf("Google authorization response = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "visto_google_state" || cookies[0].Value == "" || cookies[0].MaxAge != 600 || !cookies[0].HttpOnly {
		t.Fatalf("Google state cookie = %#v", cookies)
	}
}

func TestGoogleCallback_GivenMismatchedState_WhenReturnedFromProvider_ThenItDoesNotCreateSession(t *testing.T) {
	service := auth.NewService(&accountHTTPRepository{}, auth.Config{GoogleEnabled: true})
	config := httpserver.GoogleOAuthConfig{ClientID: "client-id", ClientSecret: "client-secret", RedirectURL: "https://visto.example/api/v1/auth/google/callback"}
	handler := httpserver.New(service, nil, "", nil, nil, nil, nil, nil, nil).WithGoogleOAuth(config).Handler()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?state=attacker-state&code=ignored", nil)
	request.AddCookie(&http.Cookie{Name: "visto_google_state", Value: "expected-state"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/?auth_error=google" {
		t.Fatalf("Google callback response = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "visto_session" {
			t.Fatal("mismatched OAuth state must not create a session")
		}
	}
}
