package sqlite_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestPagesTraverseLibraryPlaysAndUsers(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "pager", "Pager", "private")
	for _, row := range []struct{ id, title, updated string }{{"movie:1", "Alpha", "2026-01-02T00:00:00Z"}, {"movie:2", "Alpha", "2026-01-02T00:00:00Z"}, {"movie:3", "Zulu", "2026-01-01T00:00:00Z"}} {
		if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES(?,'movie',?,?,?,?)`, row.id, row.id[6:], row.title, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?, ?, ?, 'watchlist', ?, ?)`, userID+row.id, userID, row.id, testTimestamp, row.updated); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example Show',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?, ?, 'tv:42', 'watching', ?, ?)`, userID+"tv:42", userID, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	req := pagination.Request{Limit: 1}
	ids := []string{}
	for {
		page, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "updated", Status: "watchlist"}, req)
		if err != nil {
			t.Fatal(err)
		}
		if page.TotalCount == nil || *page.TotalCount != 3 {
			t.Fatalf("watchlist total_count = %v, want 3", page.TotalCount)
		}
		for _, item := range page.Items {
			ids = append(ids, item.Item.MediaID)
		}
		if page.NextCursor == nil {
			break
		}
		req.Cursor = *page.NextCursor
	}
	tvPage, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "updated", Status: "watching", MediaType: "tv"}, pagination.Request{Limit: 1})
	if err != nil || tvPage.TotalCount == nil || *tvPage.TotalCount != 1 || len(tvPage.Items) != 1 {
		t.Fatalf("filtered TV count page: %+v, %v", tvPage, err)
	}
	emptyPage, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "updated", Status: "watching", MediaType: "movie"}, pagination.Request{Limit: 1})
	if err != nil || emptyPage.TotalCount == nil || *emptyPage.TotalCount != 0 || len(emptyPage.Items) != 0 {
		t.Fatalf("empty filtered library count page: %+v, %v", emptyPage, err)
	}
	if len(ids) != 3 || ids[0] != "movie:1" || ids[1] != "movie:2" || ids[2] != "movie:3" {
		t.Fatalf("library pages: %v", ids)
	}
	if _, err := store.DB.Exec(`UPDATE media SET original_title='The First Film' WHERE id='movie:1'`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"ALPHA", []string{"movie:1", "movie:2"}},
		{"first film", []string{"movie:1"}},
		{"%", nil},
	} {
		request := pagination.Request{Limit: 1}
		var got []string
		for {
			page, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "title", Status: "watchlist", MediaType: "movie", Query: tc.query}, request)
			if err != nil {
				t.Fatal(err)
			}
			if page.TotalCount == nil || *page.TotalCount != len(tc.want) {
				t.Fatalf("query %q count = %v", tc.query, page.TotalCount)
			}
			for _, item := range page.Items {
				got = append(got, item.Item.MediaID)
			}
			if page.NextCursor == nil {
				break
			}
			request.Cursor = *page.NextCursor
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) && !(len(got) == 0 && len(tc.want) == 0) {
			t.Fatalf("query %q items = %v, want %v", tc.query, got, tc.want)
		}
	}
	searched, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "title", Status: "watchlist", Query: "alpha"}, pagination.Request{Limit: 1})
	if err != nil || searched.NextCursor == nil {
		t.Fatalf("search first page = %+v, %v", searched, err)
	}
	if _, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "title", Status: "watchlist", Query: "zulu"}, pagination.Request{Limit: 1, Cursor: *searched.NextCursor}); err == nil {
		t.Fatal("cursor accepted for a different search")
	}
	for _, tc := range []struct {
		sort string
		want []string
	}{
		{"title", []string{"movie:1", "movie:2", "movie:3"}},
		{"released", []string{"movie:1", "movie:2", "movie:3"}},
	} {
		request := pagination.Request{Limit: 1}
		got := []string{}
		for {
			page, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: tc.sort, Status: "watchlist"}, request)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range page.Items {
				got = append(got, item.Item.MediaID)
			}
			if page.NextCursor == nil {
				break
			}
			request.Cursor = *page.NextCursor
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%s pages: %v", tc.sort, got)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%s pages: %v", tc.sort, got)
			}
		}
	}
	if _, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "title"}, pagination.Request{Limit: 1, Cursor: req.Cursor}); err == nil {
		t.Fatal("cursor accepted for a different sort")
	}
	users, err := store.ListUsersPage(ctx, pagination.Request{Limit: 1})
	if err != nil || len(users.Items) != 1 {
		t.Fatalf("users page: %v %v", users, err)
	}
	if users.NextCursor != nil {
		next, err := store.ListUsersPage(ctx, pagination.Request{Limit: 1, Cursor: *users.NextCursor})
		if err != nil || len(next.Items) == 0 {
			t.Fatalf("next users page: %v %v", next, err)
		}
	}
	for i := 1; i <= 3; i++ {
		if _, err := store.DB.Exec(`INSERT INTO plays(user_id,media_id,watched_at,source,created_at) VALUES(?,'movie:1',?,'web',?)`, userID, time.Date(2026, 1, i, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`UPDATE user_media SET status='completed' WHERE user_id=? AND media_id='movie:1'`, userID); err != nil {
		t.Fatal(err)
	}
	completed, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "updated", Status: "completed"}, pagination.Request{Limit: 10})
	if err != nil || len(completed.Items) != 1 || completed.Items[0].Item.MediaID != "movie:1" {
		t.Fatalf("completed library page: %v %v", completed, err)
	}
	remaining, err := store.ListItemsPage(ctx, userID, library.ListOptions{Sort: "updated", Status: "watchlist"}, pagination.Request{Limit: 10})
	if err != nil || len(remaining.Items) != 2 {
		t.Fatalf("watchlist page after play: %v %v", remaining, err)
	}
	req = pagination.Request{Limit: 2}
	first, err := store.ListPlaysPage(ctx, userID, "movie:1", "", "", req)
	if err != nil || len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("first plays page: %v %v", first, err)
	}
	next, err := store.ListPlaysPage(ctx, userID, "movie:1", "", "", pagination.Request{Limit: 2, Cursor: *first.NextCursor})
	if err != nil || len(next.Items) != 1 || next.NextCursor != nil {
		t.Fatalf("next plays page: %v %v", next, err)
	}
}

func TestEpisodePagesRespectSeasonAndOwner(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner := insertTestUser(t, store.DB, "episode-owner", "Owner", "private")
	other := insertTestUser(t, store.DB, "episode-other", "Other", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Show',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','watching',?,?)`, owner+"tv:42", owner, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES('s1','tv:42',1,'Season 1')`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES(?, 'tv:42','s1',1,?,?)`, fmt.Sprintf("e%d", i), i, fmt.Sprintf("Episode %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.ListSeasonEpisodesPage(ctx, owner, "s1", pagination.Request{Limit: 2})
	if err != nil || len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("first episode page: %v %v", first, err)
	}
	second, err := store.ListSeasonEpisodesPage(ctx, owner, "s1", pagination.Request{Limit: 2, Cursor: *first.NextCursor})
	if err != nil || len(second.Items) != 1 || second.NextCursor != nil {
		t.Fatalf("second episode page: %v %v", second, err)
	}
	if _, err := store.ListSeasonEpisodesPage(ctx, other, "s1", pagination.Request{Limit: 2}); err == nil {
		t.Fatal("other user accessed season")
	}
	show, err := store.ListShowEpisodesPage(ctx, owner, "tv:42", pagination.Request{Limit: 2})
	if err != nil || len(show.Items) != 2 || show.NextCursor == nil {
		t.Fatalf("show episode page: %v %v", show, err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES(?,'e1',?,'web',?)`, owner, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	episodeHistory, err := store.ListPlaysPage(ctx, owner, "", "e1", "", pagination.Request{Limit: 1})
	if err != nil || len(episodeHistory.Items) != 1 || *episodeHistory.Items[0].Play.EpisodeID != "e1" {
		t.Fatalf("episode history page: %v %v", episodeHistory, err)
	}
	otherHistory, err := store.ListPlaysPage(ctx, other, "", "e1", "", pagination.Request{Limit: 1})
	if err != nil || len(otherHistory.Items) != 0 {
		t.Fatalf("other user's episode history: %v %v", otherHistory, err)
	}
}

func TestPagingQueryPlansUseBoundedIndexes(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, tc := range []struct{ query, index string }{
		{`SELECT id FROM plays WHERE user_id=1 AND media_id='movie:1' AND watched_at<'2026-01-01' ORDER BY watched_at DESC,id DESC LIMIT 31`, `idx_plays_user_media_watched`},
		{`SELECT id FROM episodes WHERE season_id='s1' AND season_number=1 AND episode_number>1 ORDER BY season_number,episode_number,id LIMIT 31`, `idx_episodes_season_paging`},
		{`SELECT id FROM users WHERE created_at>'2026-01-01' ORDER BY created_at,id LIMIT 31`, `idx_users_created_paging`},
		{`SELECT media_id FROM user_media WHERE user_id=1 AND updated_at<'2026-01-01' ORDER BY updated_at DESC,media_id LIMIT 31`, `idx_user_media_paging_updated`},
	} {
		rows, err := store.DB.QueryContext(ctx, "EXPLAIN QUERY PLAN "+tc.query)
		if err != nil {
			t.Fatal(err)
		}
		details := ""
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			details += detail
		}
		rows.Close()
		if !strings.Contains(details, tc.index) {
			t.Fatalf("plan %q does not use %s: %s", tc.query, tc.index, details)
		}
	}
}

func TestShowHistoryReturnsNewestEpisodePlayForOwnerAndShow(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner := insertTestUser(t, store.DB, "show-owner", "Owner", "private")
	other := insertTestUser(t, store.DB, "show-other", "Other", "private")
	for _, id := range []string{"42", "43", "44"} {
		if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES(?,'tv',?,'Show',?,?)`, "tv:"+id, id, testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:1','movie',1,'Movie',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	for _, show := range []string{"tv:42", "tv:43"} {
		for season := 1; season <= 2; season++ {
			seasonID := fmt.Sprintf("%s-s%d", show, season)
			if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name) VALUES(?,?,?,'Season')`, seasonID, show, season); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES(?,?,?,?,1,'Episode')`, seasonID+"-e1", show, seasonID, season); err != nil {
				t.Fatal(err)
			}
		}
	}
	play := func(user, episode string, day int) {
		t.Helper()
		at := time.Date(2026, 1, day, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
		if _, err := store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES(?,?,?,'web',?)`, user, episode, at, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	play(owner, "tv:42-s2-e1", 5)
	play(owner, "tv:42-s1-e1", 3)
	play(owner, "tv:42-s1-e1", 9) // rewatch is newest
	play(owner, "tv:43-s2-e1", 20)
	play(other, "tv:42-s2-e1", 25)
	if _, err := store.DB.Exec(`INSERT INTO plays(user_id,media_id,watched_at,source,created_at) VALUES(?,'movie:1',?,'web',?)`, owner, time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), testTimestamp); err != nil {
		t.Fatal(err)
	}

	latest, err := store.ListPlaysPage(ctx, owner, "", "", "tv:42", pagination.Request{Limit: 1})
	if err != nil || len(latest.Items) != 1 || *latest.Items[0].Play.EpisodeID != "tv:42-s1-e1" || latest.Items[0].Play.WatchedAt.Day() != 9 {
		t.Fatalf("latest show play = %#v err=%v; want the day-9 rewatch", latest.Items, err)
	}
	all, err := store.ListPlaysPage(ctx, owner, "", "", "tv:42", pagination.Request{Limit: 10})
	if err != nil || len(all.Items) != 3 {
		t.Fatalf("show plays = %d err=%v; want 3 and none from another show, user or movie", len(all.Items), err)
	}
	none, err := store.ListPlaysPage(ctx, owner, "", "", "tv:44", pagination.Request{Limit: 1})
	if err != nil || len(none.Items) != 0 {
		t.Fatalf("unwatched show = %v err=%v; want empty", none.Items, err)
	}
	episode, err := store.ListPlaysPage(ctx, owner, "", "tv:42-s2-e1", "", pagination.Request{Limit: 10})
	if err != nil || len(episode.Items) != 1 {
		t.Fatalf("episode filter = %d err=%v; want 1", len(episode.Items), err)
	}
	movie, err := store.ListPlaysPage(ctx, owner, "movie:1", "", "", pagination.Request{Limit: 10})
	if err != nil || len(movie.Items) != 1 {
		t.Fatalf("movie filter = %d err=%v; want 1", len(movie.Items), err)
	}
}
