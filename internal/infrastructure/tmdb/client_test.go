package tmdb_test

import (
	"context"
	"errors"
	"testing"

	"github.com/afonsocosta/visto/internal/infrastructure/tmdb"
)

func TestNew_GivenNoAPIKey_WhenCreatingClient_ThenItFails(t *testing.T) {
	client, err := tmdb.New("", nil)
	if err == nil {
		t.Fatal("expected empty API key to fail")
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil", client)
	}
}

func TestSearch_GivenCancelledContext_WhenSearching_ThenItDoesNotCallTMDB(t *testing.T) {
	client, err := tmdb.New("test-key", nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.Search(ctx, "Severance", "en-US")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancelled", err)
	}
}
