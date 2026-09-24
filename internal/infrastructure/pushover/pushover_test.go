package pushover

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAESGCMCipherEncryptsAndAuthenticatesUserKeys(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cipher, err := NewAESGCMCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	first, err := cipher.Encrypt("pushover-user-secret")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.Encrypt("pushover-user-secret")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || strings.Contains(first, "pushover-user-secret") {
		t.Fatal("expected randomized ciphertext with no plaintext")
	}
	got, err := cipher.Decrypt(first)
	if err != nil || got != "pushover-user-secret" {
		t.Fatalf("Decrypt() = %q, %v", got, err)
	}
	other, err := NewAESGCMCipher(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32))))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Decrypt(first); err == nil {
		t.Fatal("expected authentication failure with wrong key")
	}
}

func TestPushoverClientPostsFormAndChecksProviderStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/1/messages.json" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("token") != "app-token" || r.Form.Get("user") != "user-key" || r.Form.Get("message") != "New episode is available" {
			t.Errorf("unexpected Pushover form: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":1}`)
	}))
	defer server.Close()
	client, err := NewClient("app-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = server.URL + "/1/messages.json"
	if err := client.Send(context.Background(), "user-key", "Visto", "New episode is available"); err != nil {
		t.Fatal(err)
	}

	invalidServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"status":0,"errors":["invalid user key"]}`)
	}))
	defer invalidServer.Close()
	client.endpoint = invalidServer.URL + "/1/messages.json"
	if err := client.Send(context.Background(), "user-key", "Visto", "New episode is available"); err == nil {
		t.Fatal("expected provider error")
	}
}

func TestAESGCMCipherRejectsInvalidMasterKeys(t *testing.T) {
	for _, key := range []string{"", "not-base64", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := NewAESGCMCipher(key); err == nil {
			t.Errorf("NewAESGCMCipher(%q) unexpectedly succeeded", key)
		}
	}
}
