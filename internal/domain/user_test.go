package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

func TestUser_GivenAuthenticatedUser_WhenEncodedForTheAPI_ThenItUsesDocumentedFieldNames(t *testing.T) {
	user := domain.User{ID: "user-1", Username: "afonso", DisplayName: "Afonso", Role: domain.AdminRole, CreatedAt: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)}
	encoded, err := json.Marshal(user)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"id"`, `"username"`, `"display_name"`, `"role"`, `"created_at"`} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("encoded user %s missing %s", encoded, field)
		}
	}
}
