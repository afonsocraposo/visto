package pagination_test

import (
	"github.com/afonsocosta/visto/internal/application/pagination"
	"net/http/httptest"
	"testing"
)

func TestParseRejectsInvalidLimits(t *testing.T) {
	for _, limit := range []string{"0", "101", "nope", "-1"} {
		request := httptest.NewRequest("GET", "/items?limit="+limit, nil)
		if _, err := pagination.Parse(request); err == nil {
			t.Fatalf("accepted limit %q", limit)
		}
	}
}
func TestCursorRejectsWrongScopeAndShape(t *testing.T) {
	cursor := pagination.Encode("library|owner|updated", "2026-01-01T00:00:00Z", "Title", "movie:1")
	if _, err := pagination.Decode(cursor, "library|other|updated", 3); err == nil {
		t.Fatal("accepted cursor for another owner")
	}
	if _, err := pagination.Decode(cursor, "library|owner|updated", 2); err == nil {
		t.Fatal("accepted wrong cursor shape")
	}
}
