package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

func TestExternalRatingsRequireAuthenticationAndAllowDisabledProvider(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	handler := httpserver.New(auth.NewService(fixture.store), nil, "", nil, nil, nil, nil, nil, nil).WithExternalRatings(nil).Handler()
	for _, test := range []struct {
		path, session string
		status        int
	}{
		{"/api/v1/media/movie/10/ratings", "", 401},
		{"/api/v1/media/movie/10/ratings", "viewer-session", 200},
		{"/api/v1/media/tv/2316/ratings", "viewer-session", 200},
		{"/api/v1/media/tv/2316/seasons/1/episodes/1/ratings", "", 401},
		{"/api/v1/media/tv/2316/seasons/0/episodes/1/ratings", "viewer-session", 200},
		{"/api/v1/media/tv/2316/seasons/-1/episodes/1/ratings", "viewer-session", 400},
		{"/api/v1/media/tv/2316/seasons/1/episodes/0/ratings", "viewer-session", 400},
		{"/api/v1/media/person/10/ratings", "viewer-session", 400},
		{"/api/v1/media/movie/0/ratings", "viewer-session", 400},
	} {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		if test.session != "" {
			req.AddCookie(&http.Cookie{Name: "visto_session", Value: test.session})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != test.status {
			t.Fatalf("%s: %d %s", test.path, response.Code, response.Body)
		}
		if test.status == 200 {
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body["enabled"] != false || len(body) != 1 {
				t.Fatalf("disabled response=%s, %v", response.Body, err)
			}
		}
	}
}
