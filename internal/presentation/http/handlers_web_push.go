package httpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	push "github.com/SherClockHolmes/webpush-go"
	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	webpushclient "github.com/afonsocosta/visto/internal/infrastructure/webpush"
)

type pushStore interface {
	SavePushSubscription(context.Context, string, string, string) error
	HasPushSubscription(context.Context, string, string) (bool, error)
	DeletePushSubscription(context.Context, string, string) error
}
type pushCipher interface{ Encrypt(string) (string, error) }
type WebPushConfig struct {
	Store  pushStore
	Cipher pushCipher
	Client *webpushclient.Client
}

func validSubscription(s push.Subscription) bool {
	u, err := url.Parse(s.Endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || net.ParseIP(u.Hostname()) != nil || strings.EqualFold(u.Hostname(), "localhost") || len(s.Endpoint) > 2048 {
		return false
	}
	auth, e1 := base64.RawURLEncoding.DecodeString(s.Keys.Auth)
	key, e2 := base64.RawURLEncoding.DecodeString(s.Keys.P256dh)
	return e1 == nil && e2 == nil && len(auth) == 16 && len(key) == 65
}
func pushRoutes(mux *http.ServeMux, authService *auth.Service, cfg *WebPushConfig) {
	mux.HandleFunc("GET /api/v1/profile/web-push-key", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		if cfg == nil {
			writeError(w, 503, "Web Push is not configured")
			return
		}
		writeJSON(w, 200, map[string]string{"public_key": cfg.Client.PublicKey})
	})
	mux.HandleFunc("POST /api/v1/profile/web-push-status", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if cfg == nil {
			writeError(w, 503, "Web Push is not configured")
			return
		}
		var request struct {
			Endpoint string `json:"endpoint"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Endpoint == "" {
			writeError(w, 400, "endpoint is required")
			return
		}
		exists, err := cfg.Store.HasPushSubscription(r.Context(), user.ID, sqlite.PushSubscriptionID(request.Endpoint))
		if err != nil {
			writeError(w, 500, "could not check subscription")
			return
		}
		writeJSON(w, 200, map[string]bool{"enabled": exists})
	})
	mux.HandleFunc("POST /api/v1/profile/web-push-subscription", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if cfg == nil {
			writeError(w, 503, "Web Push is not configured")
			return
		}
		var sub push.Subscription
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil || !validSubscription(sub) {
			writeError(w, 400, "invalid push subscription")
			return
		}
		data, _ := json.Marshal(sub)
		encrypted, err := cfg.Cipher.Encrypt(string(data))
		if err != nil {
			writeError(w, 500, "could not protect subscription")
			return
		}
		if err := cfg.Store.SavePushSubscription(r.Context(), user.ID, sqlite.PushSubscriptionID(sub.Endpoint), encrypted); err != nil {
			writeError(w, 500, "could not save subscription")
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("DELETE /api/v1/profile/web-push-subscription", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if cfg == nil {
			writeError(w, 503, "Web Push is not configured")
			return
		}
		var request struct {
			Endpoint string `json:"endpoint"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Endpoint == "" {
			writeError(w, 400, "endpoint is required")
			return
		}
		if err := cfg.Store.DeletePushSubscription(r.Context(), user.ID, sqlite.PushSubscriptionID(request.Endpoint)); err != nil {
			writeError(w, 500, "could not remove subscription")
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /api/v1/profile/web-push-test", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if cfg == nil {
			writeError(w, 503, "Web Push is not configured")
			return
		}
		var sub push.Subscription
		if json.NewDecoder(r.Body).Decode(&sub) != nil || !validSubscription(sub) {
			writeError(w, 400, "invalid push subscription")
			return
		}
		exists, err := cfg.Store.HasPushSubscription(r.Context(), user.ID, sqlite.PushSubscriptionID(sub.Endpoint))
		if err != nil {
			writeError(w, 500, "could not check subscription")
			return
		}
		if !exists {
			writeError(w, 404, "this device is not subscribed")
			return
		}
		data, _ := json.Marshal(map[string]string{"title": "Visto test notification", "body": "Notifications are working on this device.", "url": "/profile", "user_id": user.ID})
		raw, _ := json.Marshal(sub)
		expired, err := cfg.Client.SendPush(r.Context(), string(raw), data)
		if expired {
			cfg.Store.DeletePushSubscription(r.Context(), user.ID, sqlite.PushSubscriptionID(sub.Endpoint))
			writeError(w, 410, "subscription expired; enable notifications again")
			return
		}
		if err != nil {
			writeError(w, 502, "push provider rejected test notification")
			return
		}
		w.WriteHeader(204)
	})
}
