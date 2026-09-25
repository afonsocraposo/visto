package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestExport_GivenTwoUsersWithSharedMedia_WhenEachExports_ThenEachReceivesOnlyTheirOwnLibraryAndPlays(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	aliceID := insertTestUser(t, store.DB, "alice", "Alice", "private")
	bobID := insertTestUser(t, store.DB, "bob", "Bob", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:10','movie',10,'Shared Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ userID, status, rating string }{
		{aliceID, "watching", "5"}, {bobID, "watchlist", ""},
	} {
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at) VALUES(?,?,?, ?,NULLIF(?,''),?,?)`, row.userID+":movie:10", row.userID, "movie:10", row.status, row.rating, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	movieID := "movie:10"
	alicePlay, err := store.CreatePlay(ctx, tracking.Play{UserID: aliceID, MediaID: &movieID, WatchedAt: time.Now().UTC(), Source: "web"})
	if err != nil {
		t.Fatal(err)
	}
	bobPlay, err := store.CreatePlay(ctx, tracking.Play{UserID: bobID, MediaID: &movieID, WatchedAt: time.Now().UTC(), Source: "web"})
	if err != nil {
		t.Fatal(err)
	}

	// When each user exports, shared catalogue metadata must not join their data.
	for _, expectation := range []struct {
		userID, expectedPlayID, otherPlayID, expectedStatus string
		wantRating                                          bool
	}{
		{aliceID, alicePlay.ID, bobPlay.ID, "watching", true},
		{bobID, bobPlay.ID, alicePlay.ID, "watchlist", false},
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
