package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

func TestHealth_GivenRunningServer_WhenHealthIsRequested_ThenItReportsOK(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	httpserver.New().Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content type = %q, want application/json", contentType)
	}
}
