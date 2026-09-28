package httpserver_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afonsocosta/visto/internal/application/importer"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

func TestImportRoutesRequireAuthenticationAndLimitUploads(t *testing.T) {
	handler := httpserver.New(nil, nil, "", nil, nil, nil, nil, nil, nil).WithImports(importer.NewService(nil, nil)).Handler()
	for _, route := range []struct{ method, path string }{{"GET", "/api/v1/imports/welcome"}, {"POST", "/api/v1/imports/welcome/dismiss"}, {"POST", "/api/v1/imports/bingers"}} {
		request := httptest.NewRequest(route.method, route.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s returned %d", route.method, route.path, response.Code)
		}
	}
	request := httptest.NewRequest("POST", "/api/v1/imports/bingers", bytes.NewReader(make([]byte, importer.MaxUpload+1)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload returned %d", response.Code)
	}
}
