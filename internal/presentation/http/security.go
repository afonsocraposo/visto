package httpserver

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const loginWindow = 15 * time.Minute
const loginFailureLimit = 5

type loginAttempts struct {
	failures     int
	windowStart  time.Time
	blockedUntil time.Time
}
type LoginLimiter struct {
	mu      sync.Mutex
	clients map[string]loginAttempts
	checks  uint64
	now     func() time.Time
}

func newLoginLimiter() *LoginLimiter {
	return &LoginLimiter{clients: map[string]loginAttempts{}, now: time.Now}
}
func (limiter *LoginLimiter) allowed(ip string) (bool, time.Duration) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	limiter.checks++
	if limiter.checks%256 == 0 {
		for key, attempt := range limiter.clients {
			if now.Sub(attempt.windowStart) > 2*loginWindow && now.After(attempt.blockedUntil) {
				delete(limiter.clients, key)
			}
		}
	}
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
	attempt.failures++
	if attempt.failures >= loginFailureLimit {
		attempt.failures = 0
		attempt.blockedUntil = now.Add(loginWindow)
		attempt.windowStart = now
	}
	limiter.clients[ip] = attempt
}
func (limiter *LoginLimiter) reset(ip string) {
	limiter.mu.Lock()
	delete(limiter.clients, ip)
	limiter.mu.Unlock()
}

func clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

func csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
			expectedScheme = "https"
		}
		if !strings.EqualFold(parsed.Scheme, expectedScheme) || !strings.EqualFold(parsed.Host, r.Host) {
			http.Error(w, "cross-origin request rejected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
