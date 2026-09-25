package httpserver

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/afonsocosta/visto/internal/presentation/security"
)

const loginWindow = 15 * time.Minute
const loginFailureLimit = 5

type loginAttempts struct {
	failures     int
	windowStart  time.Time
	blockedUntil time.Time
}
type LoginLimiter struct {
	mu         sync.Mutex
	clients    map[string]loginAttempts
	maxEntries int
	now        func() time.Time
}

func newLoginLimiter() *LoginLimiter {
	return &LoginLimiter{clients: map[string]loginAttempts{}, maxEntries: 10_000, now: time.Now}
}
func (limiter *LoginLimiter) allowed(ip string) (bool, time.Duration) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	attempt, exists := limiter.clients[ip]
	if !exists {
		return true, 0
	}
	if now.Before(attempt.blockedUntil) {
		return false, attempt.blockedUntil.Sub(now)
	}
	if !attempt.windowStart.IsZero() && now.Sub(attempt.windowStart) >= loginWindow {
		delete(limiter.clients, ip)
	}
	return true, 0
}
func (limiter *LoginLimiter) failed(ip string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	attempt := limiter.clients[ip]
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) >= loginWindow {
		attempt = loginAttempts{windowStart: now}
	}
	if _, exists := limiter.clients[ip]; !exists && len(limiter.clients) >= limiter.maxEntries {
		limiter.evictOne()
	}
	attempt.failures++
	if attempt.failures >= loginFailureLimit {
		attempt.failures = 0
		attempt.blockedUntil = now.Add(loginWindow)
		attempt.windowStart = now
	}
	limiter.clients[ip] = attempt
}

func (limiter *LoginLimiter) evictOne() {
	var oldestIP string
	var oldest time.Time
	for ip, attempt := range limiter.clients {
		seen := attempt.windowStart
		if attempt.blockedUntil.After(seen) {
			seen = attempt.blockedUntil
		}
		if oldestIP == "" || seen.Before(oldest) {
			oldestIP, oldest = ip, seen
		}
	}
	delete(limiter.clients, oldestIP)
}
func (limiter *LoginLimiter) reset(ip string) {
	limiter.mu.Lock()
	delete(limiter.clients, ip)
	limiter.mu.Unlock()
}

func csrfProtection(next http.Handler, proxies *security.ProxyResolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Plex authenticates callbacks with the per-user high-entropy path
		// secret, so browser-origin checks are not applicable to this route.
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/webhooks/plex/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
				http.Error(w, "cross-site request rejected", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			http.Error(w, "invalid request origin", http.StatusForbidden)
			return
		}
		expectedScheme := "http"
		if proxies != nil && proxies.IsHTTPS(r) || proxies == nil && r.TLS != nil {
			expectedScheme = "https"
		}
		if !strings.EqualFold(parsed.Scheme, expectedScheme) || !strings.EqualFold(parsed.Host, r.Host) {
			http.Error(w, "cross-origin request rejected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
