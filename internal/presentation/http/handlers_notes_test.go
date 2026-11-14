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

func TestEpisodeNotesArePrivateIndependentAndPreservedThroughCatalogRefresh(t *testing.T) {
	fixture := newMetadataRefreshFixture(t)
	ctx := context.Background()
	request := func(method, id, session, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/episodes/"+id+"/note", strings.NewReader(body))
		if session != "" {
			req.AddCookie(&http.Cookie{Name: "visto_session", Value: session})
		}
		response := httptest.NewRecorder()
		fixture.handler.ServeHTTP(response, req)
		return response
	}
	id := "tv:2316:episode:1"
	if got := request(http.MethodPut, id, "", `{"note":"private"}`); got.Code != 401 {
		t.Fatalf("unauthenticated=%d", got.Code)
	}
	if got := request(http.MethodPut, id, "viewer-session", `{"note":"Episode one"}`); got.Code != 200 {
		t.Fatalf("save=%d %s", got.Code, got.Body)
	}
	if got := request(http.MethodGet, id, "admin-session", ""); got.Body.String() != "{\"note\":\"\"}\n" {
		t.Fatalf("another account read private note: %s", got.Body)
	}
	if got := request(http.MethodPut, id, "admin-session", `{"note":"Admin's own note"}`); got.Code != 200 {
		t.Fatal(got.Body)
	}
	if got := request(http.MethodPut, id, "viewer-session", `{"note":"`+strings.Repeat("x", 5001)+`"}`); got.Code != 400 {
		t.Fatalf("oversized=%d", got.Code)
	}
	if got := request(http.MethodGet, "unknown", "viewer-session", ""); got.Code != 404 {
		t.Fatalf("unknown episode=%d", got.Code)
	}
	if _, err := fixture.store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,tmdb_id,name) VALUES('tv:2316:episode:2','tv:2316','tv:2316:season:1',1,2,1002,'Second')`); err != nil {
		t.Fatal(err)
	}
	if got := request(http.MethodPut, "tv:2316:episode:2", "viewer-session", `{"note":"Episode two"}`); got.Code != 200 {
		t.Fatalf("second note=%d %s", got.Code, got.Body)
	}
	if err := fixture.store.SetNote(ctx, "2", "tv:2316", "Series note"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.ImportShowMetadata(ctx, "tv:2316", domain.TVShowMetadata{TMDBID: 2316, Name: "Show", Seasons: []domain.TVSeasonMetadata{{Number: 1, Episodes: []domain.TVEpisodeMetadata{}}}}); err != nil {
		t.Fatal(err)
	}
	data, err := fixture.store.Export(ctx, "2")
	if err != nil || len(data.EpisodeNotes) != 2 || data.Library[0].Note != "Series note" {
		t.Fatalf("export=%+v err=%v", data, err)
	}
	for _, note := range data.EpisodeNotes {
		if note.Note == "Admin's own note" {
			t.Fatal("export leaked another user's note")
		}
	}
	if got := request(http.MethodPut, id, "viewer-session", `{"note":""}`); got.Code != 200 {
		t.Fatalf("delete=%d", got.Code)
	}
	if note, err := fixture.store.GetEpisodeNote(ctx, "1", id); err != nil || note != "Admin's own note" {
		t.Fatalf("delete changed other user's note: %q %v", note, err)
	}
	if note, err := fixture.store.GetEpisodeNote(ctx, "2", "tv:2316:episode:2"); err != nil || note != "Episode two" {
		t.Fatalf("delete changed sibling episode: %q %v", note, err)
	}
}

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
