package httpserver_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/plexsync"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

type plexWebhookRepository struct {
	validToken bool
	logged     []plexsync.Event
}

func (*plexWebhookRepository) IssuePlexWebhook(context.Context, string, string, time.Time) error {
	return nil
}
func (*plexWebhookRepository) RevokePlexWebhook(context.Context, string) error { return nil }
func (*plexWebhookRepository) GetPlexWebhookStatus(context.Context, string) (plexsync.Status, error) {
	return plexsync.Status{}, nil
}
func (repository *plexWebhookRepository) UserForPlexWebhook(_ context.Context, hash string, _ time.Time) (string, error) {
	secretHash := sha256.Sum256([]byte("a-secret-that-is-long-enough-to-pass-validation"))
	if !repository.validToken || hash != hex.EncodeToString(secretHash[:]) {
		return "", plexsync.ErrWebhookNotFound
	}
	return "alice", nil
}
func (repository *plexWebhookRepository) LogPlexEvent(_ context.Context, _, _ string, event plexsync.Event) error {
	repository.logged = append(repository.logged, event)
	return nil
}
func (*plexWebhookRepository) RecordPlexPlay(context.Context, string, string, plexsync.Event, *string, *string, time.Time, time.Duration) (bool, error) {
	return false, nil
}

func TestPlexWebhook_GivenExternalMultipartRequests_WhenValidated_ThenOnlySecretAuthenticatedEventsReachSync(t *testing.T) {
	secret := "a-secret-that-is-long-enough-to-pass-validation"
	repository := &plexWebhookRepository{validToken: true}
	service := plexsync.NewService(repository, nil, nil, "https://visto.example.com")
	handler := httpserver.New(nil, nil, "", nil, nil, nil, nil, nil, nil).WithPlexSync(service).Handler()

	t.Run("valid Plex multipart callback", func(t *testing.T) {
		body, contentType := plexMultipart(t, `{"event":"media.scrobble","Metadata":{"type":"movie","title":"Example"}}`)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/plex/"+secret, body)
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Origin", "https://attacker.example")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("status=%d body=%s, want 204", response.Code, response.Body.String())
		}
		if len(repository.logged) != 1 || repository.logged[0].Status != "failed" {
			t.Fatalf("logged events=%#v; missing metadata should be recorded as a failed sync", repository.logged)
		}
	})

	t.Run("invalid secret", func(t *testing.T) {
		body, contentType := plexMultipart(t, `{"event":"media.scrobble"}`)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/plex/this-is-an-invalid-secret-value-that-is-long-enough", body)
		request.Header.Set("Content-Type", contentType)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status=%d, want 404 for an unknown token", response.Code)
		}
	})

	t.Run("non multipart payload", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/plex/"+secret, bytes.NewBufferString(`{"event":"media.scrobble"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status=%d, want 400 for a non-multipart request", response.Code)
		}
	})
}

func plexMultipart(t *testing.T, payload string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("payload", payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}
