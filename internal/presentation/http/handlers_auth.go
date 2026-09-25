package httpserver

import "encoding/json"
import "errors"
import "net/http"
import "strconv"
import "strings"
import "time"
import "github.com/afonsocosta/visto/internal/application/auth"
import "github.com/afonsocosta/visto/internal/domain"
import "github.com/afonsocosta/visto/internal/presentation/security"

func bootstrapStatus(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication is not configured")
			return
		}
		available, err := service.BootstrapAvailable(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "authentication status is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"bootstrap_available": available, "signup_enabled": service.SignupEnabled(), "google_enabled": service.GoogleEnabled()})
	}
}

func signup(service *auth.Service, proxies *security.ProxyResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication is not configured")
			return
		}
		var request credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, token, expiresAt, err := service.SignUp(r.Context(), request.Email, request.Name, request.Password)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrSignupsDisabled):
				writeError(w, http.StatusForbidden, err.Error())
			case errors.Is(err, auth.ErrBootstrapIncomplete):
				writeError(w, http.StatusConflict, err.Error())
			default:
				writeError(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "visto_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expiresAt, Secure: cookieSecure(r, proxies)})
		writeJSON(w, http.StatusCreated, user)
	}
}

func createUser(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		if actor.Role != domain.AdminRole {
			writeError(w, http.StatusForbidden, "administrator access required")
			return
		}
		var request credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, err := service.CreateUser(r.Context(), request.Email, request.Name, request.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user)
	}
}

func listUsers(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		if actor.Role != domain.AdminRole {
			writeError(w, http.StatusForbidden, "administrator access required")
			return
		}
		users, err := service.Users(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list users")
			return
		}
		writeJSON(w, http.StatusOK, users)
	}
}

func updateUser(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		if actor.Role != domain.AdminRole {
			writeError(w, http.StatusForbidden, "administrator access required")
			return
		}
		var request struct {
			DisplayName string `json:"name"`
			Password    string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		keepSessionToken := ""
		if actor.ID == r.PathValue("userID") {
			if cookie, err := r.Cookie("visto_session"); err == nil {
				keepSessionToken = cookie.Value
			}
		}
		if err := service.UpdateUser(r.Context(), r.PathValue("userID"), request.DisplayName, request.Password, keepSessionToken); err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
			} else {
				writeError(w, http.StatusBadRequest, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteUser(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		if actor.Role != domain.AdminRole {
			writeError(w, http.StatusForbidden, "administrator access required")
			return
		}
		userID := r.PathValue("userID")
		if userID == actor.ID {
			writeError(w, http.StatusBadRequest, "you cannot delete your own account")
			return
		}
		if err := service.DeleteUser(r.Context(), userID); err != nil {
			switch {
			case errors.Is(err, auth.ErrUserNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			case errors.Is(err, auth.ErrLastAdministrator):
				writeError(w, http.StatusConflict, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "could not delete user")
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func bootstrap(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, err := service.Bootstrap(r.Context(), request.Email, request.Name, request.Password)
		if err != nil {
			if err == auth.ErrBootstrapComplete {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user)
	}
}

func login(service *auth.Service, limiter *LoginLimiter, proxies *security.ProxyResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if proxies != nil {
			ip = proxies.ClientIP(r)
		}
		if allowed, retryAfter := limiter.allowed(ip); !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
			writeError(w, http.StatusTooManyRequests, "too many login attempts")
			return
		}
		var request credentialsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, token, expiresAt, err := service.Login(r.Context(), request.Email, request.Password)
		if err != nil {
			limiter.failed(ip)
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		limiter.reset(ip)
		http.SetCookie(w, &http.Cookie{Name: "visto_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expiresAt, Secure: cookieSecure(r, proxies)})
		writeJSON(w, http.StatusOK, user)
	}
}

func logout(service *auth.Service, proxies *security.ProxyResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "authentication is not configured")
			return
		}
		if cookie, err := r.Cookie("visto_session"); err == nil {
			if err := service.Logout(r.Context(), cookie.Value); err != nil {
				writeError(w, http.StatusInternalServerError, "could not end session")
				return
			}
		}
		http.SetCookie(w, &http.Cookie{Name: "visto_session", Value: "", Path: "/", HttpOnly: true, Secure: cookieSecure(r, proxies), SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1})
		w.WriteHeader(http.StatusNoContent)
	}
}

func cookieSecure(r *http.Request, proxies *security.ProxyResolver) bool {
	return proxies != nil && proxies.IsHTTPS(r) || proxies == nil && r.TLS != nil
}

func currentUser(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, user)
	}
}

func authenticatedUser(w http.ResponseWriter, r *http.Request, service *auth.Service) (domain.User, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Add("Vary", "Cookie")
	w.Header().Add("Vary", "Authorization")
	if service == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return domain.User{}, false
	}
	var user domain.User
	var err error
	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return domain.User{}, false
		}
		user, err = service.AuthenticatePersonalToken(r.Context(), parts[1])
	} else {
		cookie, cookieErr := r.Cookie("visto_session")
		if cookieErr != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return domain.User{}, false
		}
		user, err = service.Authenticate(r.Context(), cookie.Value)
	}
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return domain.User{}, false
	}
	return user, true
}

func listPersonalTokens(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		tokens, err := service.PersonalTokens(r.Context(), user.ID)
		if err != nil {
			status := http.StatusInternalServerError
			message := "could not list personal API tokens"
			if errors.Is(err, auth.ErrPersonalTokensUnavailable) {
				status = http.StatusServiceUnavailable
				message = "personal API tokens are not configured"
			}
			writeError(w, status, message)
			return
		}
		writeJSON(w, http.StatusOK, tokens)
	}
}

func createPersonalToken(service *auth.Service) http.HandlerFunc {
	type requestBody struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		var body requestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		var expiresAt *time.Time
		if strings.TrimSpace(body.ExpiresAt) != "" {
			parsed, err := time.Parse(time.RFC3339, body.ExpiresAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "expires_at must be an RFC 3339 timestamp")
				return
			}
			expiresAt = &parsed
		}
		token, err := service.CreatePersonalToken(r.Context(), user.ID, body.Name, expiresAt)
		if err != nil {
			status := http.StatusInternalServerError
			message := "could not create personal API token"
			if errors.Is(err, auth.ErrInvalidPersonalToken) {
				status = http.StatusBadRequest
				message = err.Error()
			} else if errors.Is(err, auth.ErrPersonalTokensUnavailable) {
				status = http.StatusServiceUnavailable
				message = "personal API tokens are not configured"
			}
			writeError(w, status, message)
			return
		}
		writeJSON(w, http.StatusCreated, token)
	}
}

func revokePersonalToken(service *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, service)
		if !ok {
			return
		}
		if err := service.RevokePersonalToken(r.Context(), user.ID, r.PathValue("tokenID")); err != nil {
			if errors.Is(err, auth.ErrPersonalTokenMissing) {
				writeError(w, http.StatusNotFound, "personal API token not found")
				return
			}
			status := http.StatusInternalServerError
			if errors.Is(err, auth.ErrPersonalTokensUnavailable) {
				status = http.StatusServiceUnavailable
			}
			writeError(w, status, "could not revoke personal API token")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
