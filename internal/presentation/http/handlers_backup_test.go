package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/backup"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

type backupCipher struct{}

func (backupCipher) Encrypt(s string) (string, error) { return "encrypted:" + s, nil }
func (backupCipher) Decrypt(s string) (string, error) {
	return strings.TrimPrefix(s, "encrypted:"), nil
}
func TestBackupSettingsRequireAdminAndNeverReturnSecret(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := &backup.Service{Store: store, Cipher: backupCipher{}, DefaultInterval: 24 * time.Hour}
	repository := &accountHTTPRepository{actor: domain.User{ID: "user-1", Role: domain.UserRole}}
	handler := httpserver.New(auth.NewService(repository), nil, "", nil, nil, nil, nil, nil, nil).WithBackups(service).Handler()
	for _, route := range []struct{ method, path string }{{"GET", "/api/v1/admin/backups"}, {"PUT", "/api/v1/admin/backups"}, {"POST", "/api/v1/admin/backups/test"}, {"POST", "/api/v1/admin/backups/run"}} {
		request := httptest.NewRequest(route.method, "http://visto.local"+route.path, nil)
		request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 403 {
			t.Fatalf("%s %s status=%d", route.method, route.path, response.Code)
		}
	}
	repository.actor.Role = domain.AdminRole
	request := httptest.NewRequest("PUT", "http://visto.local/api/v1/admin/backups", strings.NewReader(`{"destination":"s3","interval_seconds":86400,"bucket":"bucket","region":"us-east-1","access_key_id":"id","secret_key":"top-secret","prefix":"visto/","max_keep":30}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("save status=%d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "top-secret") || strings.Contains(response.Body.String(), "encrypted:") {
		t.Fatal("secret exposed in response")
	}
	saved, _ := service.Settings(ctx)
	if saved.SecretCiphertext != "encrypted:top-secret" {
		t.Fatal("secret was not encrypted")
	}
}
