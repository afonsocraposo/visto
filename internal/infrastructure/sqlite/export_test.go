package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestExport_GivenTwoUsersWithSharedMedia_WhenEachExports_ThenEachReceivesOnlyTheirOwnLibraryAndPlays(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, userID := range []string{"alice", "bob"} {
		if _, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(?,?,?,'hash','user',?,?)`, userID, userID, userID, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Shared Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ userID, status, rating string }{
		{"alice", "watching", "5"}, {"bob", "watchlist", ""},
	} {
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at) VALUES(?,?,?, ?,NULLIF(?,''),?,?)`, row.userID+":movie:10", row.userID, "movie:10", row.status, row.rating, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(id,user_id,media_id,watched_at,source,created_at) VALUES
		('alice-play','alice','movie:10',?,'web',?),('bob-play','bob','movie:10',?,'web',?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}

	// When each user exports, shared catalogue metadata must not join their data.
	for _, expectation := range []struct {
		userID, expectedPlayID, otherPlayID, expectedStatus string
		wantRating                                          bool
	}{
		{"alice", "alice-play", "bob-play", "watching", true},
		{"bob", "bob-play", "alice-play", "watchlist", false},
	} {
		data, err := store.Export(ctx, expectation.userID)
		if err != nil {
			t.Fatalf("export for %s: %v", expectation.userID, err)
		}
		if len(data.Library) != 1 || data.Library[0].MediaID != "movie:10" || data.Library[0].Status != expectation.expectedStatus {
			t.Fatalf("library export for %s=%+v", expectation.userID, data.Library)
		}
		if (data.Library[0].Rating != nil) != expectation.wantRating {
			t.Fatalf("rating privacy for %s: %+v", expectation.userID, data.Library[0])
		}
		if len(data.Plays) != 1 || data.Plays[0].ID != expectation.expectedPlayID {
			t.Fatalf("play export for %s=%+v", expectation.userID, data.Plays)
		}
		if data.Plays[0].ID == expectation.otherPlayID {
			t.Fatalf("export for %s included another user's play %s", expectation.userID, expectation.otherPlayID)
		}
	}
}
