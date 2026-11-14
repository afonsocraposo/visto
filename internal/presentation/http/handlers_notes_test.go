package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
)

func TestTitleNotePrivacyPersistenceAndValidation(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	path := "/api/v1/library/tv:2316/note"
	request := func(method, session, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if session != "" {
			req.AddCookie(&http.Cookie{Name: "visto_session", Value: session})
		}
		recorder := httptest.NewRecorder()
		fixture.handler.ServeHTTP(recorder, req)
		return recorder
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		if got := request(method, "", `{"note":"secret"}`); got.Code != 401 {
			t.Fatalf("unauthenticated %s = %d", method, got.Code)
		}
		if got := request(method, "admin-session", `{"note":"secret"}`); got.Code != 404 {
			t.Fatalf("other user %s = %d", method, got.Code)
		}
	}
	text := "Private thoughts\n<script>alert('text')</script> 🎬"
	body, _ := json.Marshal(map[string]string{"note": text})
	if got := request(http.MethodPut, "viewer-session", string(body)); got.Code != 200 {
		t.Fatalf("save = %d %s", got.Code, got.Body)
	}
	for _, body := range []string{`{}`, `{"note":null}`, `{"note":42}`, `{"note":"` + strings.Repeat("x", 5001) + `"}`} {
		if got := request(http.MethodPut, "viewer-session", body); got.Code != 400 {
			t.Fatalf("invalid note = %d", got.Code)
		}
	}
	// Updating the rating/status must preserve the note.
	_, err := library.NewService(fixture.store).Save(context.Background(), "2", "tv:2316", domain.PausedStatus, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := request(http.MethodGet, "viewer-session", "")
	var note struct {
		Note string `json:"note"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &note); err != nil || got.Code != 200 || note.Note != text {
		t.Fatalf("read = %d %s", got.Code, got.Body)
	}
	exported, err := fixture.store.Export(context.Background(), "2")
	if err != nil || len(exported.Library) != 1 || exported.Library[0].Note != text {
		t.Fatalf("note export = %+v, %v", exported, err)
	}
	if got := request(http.MethodPut, "viewer-session", `{"note":""}`); got.Code != 200 {
		t.Fatalf("delete = %d", got.Code)
	}
	if got := request(http.MethodGet, "viewer-session", ""); got.Body.String() != "{\"note\":\"\"}\n" {
		t.Fatalf("cleared note = %s", got.Body)
	}
}
