package mcp

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/oauth"
	"github.com/afonsocosta/visto/internal/presentation/security"
)

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
