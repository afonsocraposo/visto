package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/presentation/security"
)

func TestLoginLimiterBDD(t *testing.T) {
	t.Run("Given five failed attempts, When the same client tries again, Then login is blocked for the window", func(t *testing.T) {
		limiter := newLoginLimiter()
		now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		limiter.now = func() time.Time { return now }

		for range loginFailureLimit {
			limiter.failed("192.0.2.10")
		}
		allowed, retryAfter := limiter.allowed("192.0.2.10")
		if allowed {
			t.Fatal("expected login attempts to be blocked")
		}
		if retryAfter != loginWindow {
			t.Fatalf("expected retry after %s, got %s", loginWindow, retryAfter)
		}

		now = now.Add(loginWindow)
		if allowed, _ := limiter.allowed("192.0.2.10"); !allowed {
			t.Fatal("expected login to be allowed after the block expires")
		}
	})

	t.Run("Given a blocked client, When a successful login resets the limiter, Then the next attempt is allowed", func(t *testing.T) {
		limiter := newLoginLimiter()
		for range loginFailureLimit {
			limiter.failed("192.0.2.11")
		}
		limiter.reset("192.0.2.11")
		if allowed, _ := limiter.allowed("192.0.2.11"); !allowed {
			t.Fatal("expected reset client to be allowed")
		}
	})

	t.Run("Given more clients than capacity, When they fail authentication, Then the limiter remains bounded", func(t *testing.T) {
		limiter := newLoginLimiter()
		limiter.maxEntries = 2
		limiter.failed("192.0.2.1")
		limiter.failed("192.0.2.2")
		limiter.failed("192.0.2.3")
		if len(limiter.clients) != 2 {
			t.Fatalf("tracked clients = %d, want 2", len(limiter.clients))
		}
	})
}

func TestLoginRateLimitResponseBDD(t *testing.T) {
	t.Run("Given a client at the failure limit, When it submits login, Then the API returns 429 with Retry-After", func(t *testing.T) {
		limiter := newLoginLimiter()
		for range loginFailureLimit {
			limiter.failed("192.0.2.12")
		}
		handler := login(nil, limiter, &security.ProxyResolver{})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"family","password":"wrong"}`))
		request.RemoteAddr = "192.0.2.12:12345"
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)
		if response.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", response.Code)
		}
		if response.Header().Get("Retry-After") == "" {
			t.Fatal("expected a Retry-After header")
		}
	})
}

func TestCSRFProtectionBDD(t *testing.T) {
	t.Run("Given a cross-origin form request, When it reaches the API, Then it is rejected", func(t *testing.T) {
		called := false
		handler := csrfProtection(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), &security.ProxyResolver{})
		request := httptest.NewRequest(http.MethodPost, "http://visto.local/api/v1/plays", nil)
		request.Header.Set("Origin", "http://attacker.local")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", response.Code)
		}
		if called {
			t.Fatal("cross-origin request reached the protected handler")
		}
	})

	t.Run("Given a same-origin request behind TLS termination, When it reaches the API, Then it is allowed", func(t *testing.T) {
		called := false
		proxies, err := security.ParseTrustedProxies("192.0.2.0/24")
		if err != nil {
			t.Fatalf("parse proxies: %v", err)
		}
		handler := csrfProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}), &proxies)
		request := httptest.NewRequest(http.MethodPatch, "http://visto.local/api/v1/profile", nil)
		request.Header.Set("Origin", "https://visto.local")
		request.Header.Set("X-Forwarded-Proto", "https")
		request.RemoteAddr = "192.0.2.10:443"
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent || !called {
			t.Fatalf("expected same-origin request to pass, status=%d called=%t", response.Code, called)
		}
	})

	t.Run("Given a cross-site fetch without Origin, When it reaches the API, Then it is rejected", func(t *testing.T) {
		handler := csrfProtection(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("cross-site request reached the protected handler")
		}), &security.ProxyResolver{})
		request := httptest.NewRequest(http.MethodPost, "http://visto.local/api/v1/plays", nil)
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", response.Code)
		}
	})
}
