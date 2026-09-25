package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

func TestOnboardingHTTP_GivenNoAdministrator_WhenSignupIsRequested_ThenSetupMustHappenFirst(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := auth.NewService(store)
	handler := httpserver.New(service, nil, "", nil, nil, nil, nil, nil, nil).Handler()

	status := httptest.NewRecorder()
	handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"bootstrap_available":true`) || !strings.Contains(status.Body.String(), `"signup_enabled":true`) {
		t.Fatalf("fresh instance status = %d %s", status.Code, status.Body.String())
	}

	signup := httptest.NewRecorder()
	handler.ServeHTTP(signup, httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader(`{"email":"family@example.com","name":"Family","password":"a-long-family-password"}`)))
	if signup.Code != http.StatusConflict {
		t.Fatalf("pre-setup signup status = %d, want 409: %s", signup.Code, signup.Body.String())
	}
	if _, err := service.Bootstrap(ctx, "owner@example.com", "Owner", "a-long-admin-password"); err != nil {
		t.Fatal(err)
	}
	adminLogin := httptest.NewRecorder()
	handler.ServeHTTP(adminLogin, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"owner@example.com","password":"a-long-admin-password"}`)))
	if adminLogin.Code != http.StatusOK {
		t.Fatalf("admin login status = %d: %s", adminLogin.Code, adminLogin.Body.String())
	}
	adminCookie := adminLogin.Result().Cookies()[0]
	adminRequest := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.AddCookie(adminCookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	if response := adminRequest(http.MethodGet, "/api/v1/users", ""); response.Code != http.StatusOK {
		t.Fatalf("list users status = %d: %s", response.Code, response.Body.String())
	}
	created := adminRequest(http.MethodPost, "/api/v1/users", `{"email":"managed@example.com","name":"Managed","password":"a-long-managed-password"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("admin create status = %d: %s", created.Code, created.Body.String())
	}
	var managed domain.User
	if err := json.Unmarshal(created.Body.Bytes(), &managed); err != nil {
		t.Fatal(err)
	}
	if response := adminRequest(http.MethodPatch, "/api/v1/users/"+managed.ID, `{"name":"Managed User","password":"a-new-managed-password"}`); response.Code != http.StatusNoContent {
		t.Fatalf("admin update status = %d: %s", response.Code, response.Body.String())
	}
	if response := adminRequest(http.MethodDelete, "/api/v1/users/"+managed.ID, ""); response.Code != http.StatusNoContent {
		t.Fatalf("admin delete status = %d: %s", response.Code, response.Body.String())
	}

	signup = httptest.NewRecorder()
	handler.ServeHTTP(signup, httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader(`{"email":"family@example.com","name":"Family","password":"a-long-family-password"}`)))
	if signup.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, want 201: %s", signup.Code, signup.Body.String())
	}
	cookies := signup.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "visto_session" || !cookies[0].HttpOnly {
		t.Fatalf("signup session cookie = %#v", cookies)
	}
	regularRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	regularRequest.AddCookie(cookies[0])
	regularResponse := httptest.NewRecorder()
	handler.ServeHTTP(regularResponse, regularRequest)
	if regularResponse.Code != http.StatusForbidden {
		t.Fatalf("regular user admin access status = %d, want 403", regularResponse.Code)
	}

	disabled := auth.NewService(store, auth.Config{AllowSignups: false})
	disabledHandler := httpserver.New(disabled, nil, "", nil, nil, nil, nil, nil, nil).Handler()
	denied := httptest.NewRecorder()
	disabledHandler.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader(`{"email":"other@example.com","name":"Other","password":"a-long-other-password"}`)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("disabled signup status = %d, want 403", denied.Code)
	}
}
