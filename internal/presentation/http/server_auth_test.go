package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

func TestCreateUser_GivenAdministratorSession_WhenCreatingAnAccount_ThenItCreatesARegularUser(t *testing.T) {
	repository := &accountHTTPRepository{actor: domain.User{ID: "admin-1", Role: domain.AdminRole}}
	handler := httpserver.New(auth.NewService(repository), nil, "", nil, nil, nil, nil, nil, nil).Handler()
	request := httptest.NewRequest(http.MethodPost, "http://visto.local/api/v1/users", strings.NewReader(`{"username":"family","display_name":"Family Member","password":"correct-horse-battery-staple"}`))
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if repository.createdUser.Role != domain.UserRole || repository.createdUser.Username != "family" {
		t.Fatalf("created account = %+v, want regular user family", repository.createdUser)
	}
	if repository.passwordHash == "" || strings.Contains(repository.passwordHash, "correct-horse-battery-staple") {
		t.Fatal("new account password was not stored as a hash")
	}
}

func TestCreateUser_GivenRegularUserSession_WhenCreatingAnAccount_ThenItIsForbidden(t *testing.T) {
	repository := &accountHTTPRepository{actor: domain.User{ID: "user-1", Role: domain.UserRole}}
	handler := httpserver.New(auth.NewService(repository), nil, "", nil, nil, nil, nil, nil, nil).Handler()
	request := httptest.NewRequest(http.MethodPost, "http://visto.local/api/v1/users", strings.NewReader(`{"username":"family","display_name":"Family Member","password":"correct-horse-battery-staple"}`))
	request.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if repository.createdUser.ID != "" {
		t.Fatalf("regular user created an account: %+v", repository.createdUser)
	}
}

func TestPersonalAPIToken_GivenAuthenticatedUser_WhenCreatedUsedListedAndRevoked_ThenSecretIsOneTimeAndBearerAccessEnds(t *testing.T) {
	repository := &personalTokenHTTPRepository{accountHTTPRepository: &accountHTTPRepository{actor: domain.User{ID: "user-1", Username: "family", Role: domain.UserRole}}}
	service := auth.NewService(repository)
	handler := httpserver.New(service, nil, "", nil, nil, nil, nil, nil, nil).Handler()
	createRequest := httptest.NewRequest(http.MethodPost, "http://visto.local/api/v1/tokens", strings.NewReader(`{"name":"home dashboard"}`))
	createRequest.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", createResponse.Code, createResponse.Body.String())
	}
	var issued auth.IssuedPersonalToken
	if err := json.Unmarshal(createResponse.Body.Bytes(), &issued); err != nil {
		t.Fatalf("decode issued token: %v", err)
	}
	if issued.Token == "" || !strings.HasPrefix(issued.Token, "visto_pat_") {
		t.Fatalf("issued token response = %+v", issued)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "http://visto.local/api/v1/tokens", nil)
	listRequest.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || strings.Contains(listResponse.Body.String(), issued.Token) {
		t.Fatalf("list response leaked token or failed: status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}

	useRequest := httptest.NewRequest(http.MethodGet, "http://visto.local/api/v1/me", nil)
	useRequest.Header.Set("Authorization", "Bearer "+issued.Token)
	useResponse := httptest.NewRecorder()
	handler.ServeHTTP(useResponse, useRequest)
	if useResponse.Code != http.StatusOK || !strings.Contains(useResponse.Body.String(), `"id":"user-1"`) {
		t.Fatalf("bearer auth status=%d body=%s", useResponse.Code, useResponse.Body.String())
	}

	revokeRequest := httptest.NewRequest(http.MethodDelete, "http://visto.local/api/v1/tokens/"+issued.ID, nil)
	revokeRequest.AddCookie(&http.Cookie{Name: "visto_session", Value: "session-1"})
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d body=%s", revokeResponse.Code, revokeResponse.Body.String())
	}
	useAfterRevoke := httptest.NewRecorder()
	handler.ServeHTTP(useAfterRevoke, useRequest)
	if useAfterRevoke.Code != http.StatusUnauthorized {
		t.Fatalf("bearer auth after revocation status=%d, want 401", useAfterRevoke.Code)
	}
}
