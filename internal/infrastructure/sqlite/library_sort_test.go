package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestLibrarySort(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "sort-user", "Sort User", "private")
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,release_date,metadata_updated_at,created_at) VALUES
		('movie:1','movie',1,'Zulu','2020-01-01',?,?),
		('movie:2','movie',2,'Alpha','2024-01-01',?,?),
		('movie:3','movie',3,'Beta','',?,?)`, testTimestamp, testTimestamp, testTimestamp, testTimestamp, testTimestamp, testTimestamp)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, updated string }{
		{"movie:1", "2024-01-01T00:00:00Z"},
		{"movie:2", "2022-01-01T00:00:00Z"},
		{"movie:3", "2023-01-01T00:00:00Z"},
	} {
		_, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,?,'watchlist',?,?)`, userID+":"+item.id, userID, item.id, testTimestamp, item.updated)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		sort string
		want []string
	}{
		{"updated", []string{"movie:1", "movie:3", "movie:2"}},
		{"title", []string{"movie:2", "movie:3", "movie:1"}},
		{"released", []string{"movie:2", "movie:1", "movie:3"}},
	} {
		entries, err := store.ListItemsSorted(ctx, userID, tc.sort)
		if err != nil {
			t.Fatal(err)
		}
		for i, entry := range entries {
			if entry.Item.MediaID != tc.want[i] {
				t.Fatalf("sort %s position %d = %s, want %s", tc.sort, i, entry.Item.MediaID, tc.want[i])
			}
		}
	}
}
