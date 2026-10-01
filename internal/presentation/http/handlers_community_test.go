package httpserver_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/feed"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/profile"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

func TestCommunityProfiles_VisibilityAndPublicProjection(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stamp := "2026-09-24T00:00:00Z"
	for _, user := range []struct{ id, name, visibility string }{
		{"1", "Viewer", "instance"},
		{"2", "Owner", "private"},
	} {
		if _, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at,email) VALUES(?,?,?,'hash','user',?,?,?)`, user.id, user.name, user.name, stamp, stamp, user.name+"@example.test"); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB.Exec(`INSERT INTO user_settings(user_id,activity_visibility,created_at,updated_at) VALUES(?,?,?,?)`, user.id, user.visibility, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	hash := sha256.Sum256([]byte("viewer-session"))
	if _, err := store.DB.Exec(`INSERT INTO sessions(user_id,token_hash,expires_at,created_at) VALUES(1,?,'2099-01-01T00:00:00Z',?)`, hex.EncodeToString(hash[:]), stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Example Movie',?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,rating,notifications_enabled,added_at,updated_at) VALUES('item-2',2,'movie:10','watchlist',5,1,?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO activity_events(user_id,kind,media_id,occurred_at,created_at) VALUES(2,'watch','movie:10',?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO activity_events(user_id,kind,media_id,rating,occurred_at,created_at) VALUES(2,'rating','movie:10',5,'2026-09-25T00:00:00Z',?)`, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO activity_events(user_id,kind,media_id,occurred_at,created_at) VALUES(1,'watch','movie:10','2026-09-26T00:00:00Z',?)`, stamp); err != nil {
		t.Fatal(err)
	}
	handler := httpserver.New(auth.NewService(store), nil, "", library.NewService(store), nil, profile.NewService(store), feed.NewService(store), nil, nil).Handler()
	get := func(path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(&http.Cookie{Name: "visto_session", Value: "viewer-session"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	directory := get("/api/v1/community/users?limit=1")
	if directory.Code != http.StatusOK || strings.Contains(directory.Body.String(), "email") || strings.Contains(directory.Body.String(), "role") {
		t.Fatalf("directory response = %d %s", directory.Code, directory.Body.String())
	}
	var firstPage struct {
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.Unmarshal(directory.Body.Bytes(), &firstPage); err != nil || firstPage.NextCursor == nil {
		t.Fatalf("directory pagination = %s: %v", directory.Body.String(), err)
	}
	if second := get("/api/v1/community/users?limit=1&cursor=" + *firstPage.NextCursor); second.Code != http.StatusOK || !strings.Contains(second.Body.String(), "Owner") {
		t.Fatalf("second directory page = %d %s", second.Code, second.Body.String())
	}
	if response := get("/api/v1/community/users/2"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"2"`) || !strings.Contains(response.Body.String(), `"name":"Owner"`) || !strings.Contains(response.Body.String(), `"sharing":false`) || strings.Contains(response.Body.String(), "email") {
		t.Fatalf("private profile = %d %s", response.Code, response.Body.String())
	}
	for _, path := range []string{"/api/v1/community/users/2/library", "/api/v1/community/users/2/activity"} {
		if response := get(path); response.Code != http.StatusForbidden {
			t.Fatalf("private %s = %d %s", path, response.Code, response.Body.String())
		}
	}
	if response := get("/api/v1/community/users/999"); response.Code != http.StatusNotFound {
		t.Fatalf("unknown profile = %d %s", response.Code, response.Body.String())
	}
	if _, err := store.DB.Exec(`UPDATE user_settings SET activity_visibility='instance' WHERE user_id=2`); err != nil {
		t.Fatal(err)
	}
	if response := get("/api/v1/community/users/2/library"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Example Movie") || strings.Contains(response.Body.String(), "notifications_enabled") || strings.Contains(response.Body.String(), "user_id") {
		t.Fatalf("shared library = %d %s", response.Code, response.Body.String())
	}
	if response := get("/api/v1/community/users/2/library?q=example"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Example Movie") {
		t.Fatalf("searched shared library = %d %s", response.Code, response.Body.String())
	}
	if response := get("/api/v1/community/users/2/library?q=nothing-matches"); response.Code != http.StatusOK || strings.Contains(response.Body.String(), "Example Movie") {
		t.Fatalf("unmatched shared library = %d %s", response.Code, response.Body.String())
	}
	if response := get("/api/v1/community/users/2/activity"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"user_id":"2"`) || strings.Contains(response.Body.String(), `"user_id":"1"`) {
		t.Fatalf("shared activity = %d %s", response.Code, response.Body.String())
	}
	firstActivity := get("/api/v1/community/users/2/activity?limit=1")
	var activityPage struct {
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.Unmarshal(firstActivity.Body.Bytes(), &activityPage); firstActivity.Code != http.StatusOK || err != nil || activityPage.NextCursor == nil {
		t.Fatalf("first activity page = %d %s: %v", firstActivity.Code, firstActivity.Body.String(), err)
	}
	if secondActivity := get("/api/v1/community/users/2/activity?limit=1&cursor=" + *activityPage.NextCursor); secondActivity.Code != http.StatusOK || !strings.Contains(secondActivity.Body.String(), `"kind":"watch"`) {
		t.Fatalf("second activity page = %d %s", secondActivity.Code, secondActivity.Body.String())
	}
	if _, err := store.DB.Exec(`UPDATE user_settings SET activity_visibility='private' WHERE user_id=2`); err != nil {
		t.Fatal(err)
	}
	if response := get("/api/v1/community/users/2/library"); response.Code != http.StatusForbidden {
		t.Fatalf("revoked library = %d %s", response.Code, response.Body.String())
	}
	if response := get("/api/v1/community/users/2/activity"); response.Code != http.StatusForbidden {
		t.Fatalf("revoked activity = %d %s", response.Code, response.Body.String())
	}
}
