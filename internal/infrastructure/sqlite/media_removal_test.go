package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestRemoveMediaAndHistory_RemovesOnlySelectedUsersRecords(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	alice := insertTestUser(t, store.DB, "alice", "Alice", "instance")
	bob := insertTestUser(t, store.DB, "bob", "Bob", "instance")
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES
		('movie:10','movie',10,'Movie',?,?),('tv:42','tv',42,'Show',?,?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('tv:42:season:1','tv:42',1,'Season')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES
		('tv:42:episode:1','tv:42','tv:42:season:1',1,1,'One'),
		('tv:42:episode:2','tv:42','tv:42:season:1',1,2,'Two')`)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{alice, bob} {
		for _, mediaID := range []string{"movie:10", "tv:42"} {
			_, err = store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,rating,added_at,updated_at) VALUES(?,?,?,'watching',4,?,?)`, user+":"+mediaID, user, mediaID, testTimestamp, testTimestamp)
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, episodeID := range []string{"tv:42:episode:1", "tv:42:episode:2"} {
			_, err = store.DB.Exec(`INSERT INTO episode_ratings(user_id,episode_id,rating,created_at,updated_at) VALUES(?,?,5,?,?)`, user, episodeID, testTimestamp, testTimestamp)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, user := range []string{alice, bob} {
		movie := "movie:10"
		if _, err := store.CreatePlay(ctx, tracking.Play{UserID: user, MediaID: &movie, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
			t.Fatal(err)
		}
		for _, episodeID := range []string{"tv:42:episode:1", "tv:42:episode:2"} {
			if _, err := store.CreatePlay(ctx, tracking.Play{UserID: user, EpisodeID: &episodeID, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Include a bulk activity record whose play ID is stored only in JSON.
	var episodePlayID int
	if err := store.DB.QueryRow(`SELECT id FROM plays WHERE user_id=? AND episode_id='tv:42:episode:1'`, alice).Scan(&episodePlayID); err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO activity_events(user_id,kind,detail_json,occurred_at,created_at) VALUES(?,'bulk_watch',json_object('play_ids',json_array(?)),?,?)`, alice, episodePlayID, testTimestamp, testTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO activity_events(user_id,kind,media_id,rating,occurred_at,created_at) VALUES(?,'rating','movie:10',4,?,?)`, alice, testTimestamp, testTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO activity_events(user_id,kind,episode_id,rating,occurred_at,created_at) VALUES(?,'rating','tv:42:episode:1',5,?,?)`, alice, testTimestamp, testTimestamp)
	if err != nil {
		t.Fatal(err)
	}

	service := tracking.NewService(store)
	if _, err := service.RemoveMediaAndHistory(ctx, alice, "invalid"); err == nil {
		t.Fatal("invalid media ID accepted")
	}
	if _, err := service.RemoveMediaAndHistory(ctx, alice, "tv:99"); !errors.Is(err, library.ErrMediaNotFound) {
		t.Fatalf("missing title error = %v", err)
	}
	show, err := service.RemoveMediaAndHistory(ctx, alice, "tv:42")
	if err != nil || show.DeletedPlays != 2 || show.DeletedRatings != 3 {
		t.Fatalf("remove show = %+v, %v", show, err)
	}
	var remainingMoviePlays, remainingMovieActivity int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE user_id=? AND media_id='movie:10'`, alice).Scan(&remainingMoviePlays); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM activity_events WHERE user_id=? AND media_id='movie:10'`, alice).Scan(&remainingMovieActivity); err != nil {
		t.Fatal(err)
	}
	if remainingMoviePlays != 1 || remainingMovieActivity != 2 {
		t.Fatalf("unrelated movie plays/activity = %d/%d, want 1/2", remainingMoviePlays, remainingMovieActivity)
	}
	movie, err := service.RemoveMediaAndHistory(ctx, alice, "movie:10")
	if err != nil || movie.DeletedPlays != 1 || movie.DeletedRatings != 1 {
		t.Fatalf("remove movie = %+v, %v", movie, err)
	}
	for _, check := range []struct {
		query string
		args  []any
		want  int
	}{
		{`SELECT COUNT(*) FROM user_media WHERE user_id=?`, []any{alice}, 0},
		{`SELECT COUNT(*) FROM plays WHERE user_id=?`, []any{alice}, 0},
		{`SELECT COUNT(*) FROM episode_ratings WHERE user_id=?`, []any{alice}, 0},
		{`SELECT COUNT(*) FROM activity_events WHERE user_id=?`, []any{alice}, 0},
		{`SELECT COUNT(*) FROM user_media WHERE user_id=?`, []any{bob}, 2},
		{`SELECT COUNT(*) FROM plays WHERE user_id=?`, []any{bob}, 3},
		{`SELECT COUNT(*) FROM episode_ratings WHERE user_id=?`, []any{bob}, 2},
		{`SELECT COUNT(*) FROM media`, nil, 2},
		{`SELECT COUNT(*) FROM episodes`, nil, 2},
	} {
		var count int
		if err := store.DB.QueryRow(check.query, check.args...).Scan(&count); err != nil || count != check.want {
			t.Fatalf("%s: count=%d, err=%v, want %d", check.query, count, err, check.want)
		}
	}
}
