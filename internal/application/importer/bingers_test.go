package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
)

type wakeRepository struct{}

func (wakeRepository) ImportData(context.Context, string, Data) (Result, error)   { return Result{}, nil }
func (wakeRepository) ImportWelcomePending(context.Context, string) (bool, error) { return false, nil }
func (wakeRepository) DismissImportWelcome(context.Context, string) error         { return nil }

func TestImportWakesMetadataBackfillAfterSuccessfulMerge(t *testing.T) {
	archive := testArchive(t, map[string]string{
		"library.csv": "type,title,original_title,year,tvdb_id,tmdb_id,list_status,added_at\nshow,Example,Example,2020,12,42,watching,2026-01-02T03:04:05Z\n",
		"watches.csv": "type,title,tvdb_id,tmdb_id,season_number,episode_number,last_watched_at\n",
	})
	wakes := 0
	service := NewService(wakeRepository{}, nil, func() { wakes++ })
	if _, err := service.Import(context.Background(), "owner", "bingers", archive); err != nil {
		t.Fatal(err)
	}
	if wakes != 1 {
		t.Fatalf("wakes=%d", wakes)
	}
	if _, err := service.Import(context.Background(), "owner", "bingers", []byte("bad")); err == nil {
		t.Fatal("bad archive accepted")
	}
	if wakes != 1 {
		t.Fatalf("bad import woke worker: %d", wakes)
	}
}

func testArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, contents := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestParseBingers(t *testing.T) {
	archive := testArchive(t, map[string]string{
		"library.csv": "\ufefftype,title,original_title,year,tvdb_id,tmdb_id,list_status,added_at\nshow,Example,Example,2020,12,42,stopped,2026-01-02T03:04:05Z\n",
		"watches.csv": "type,title,tvdb_id,tmdb_id,season_number,episode_number,last_watched_at\nepisode,Example,12,42,1,2,2026-01-03T03:04:05Z\n",
		"ratings.csv": "type,title,tvdb_id,tmdb_id,season_number,episode_number,rating\nepisode,Example,12,42,1,2,4\nepisode,Example,12,42,1,3,9\n",
		"lists.csv":   "list_name,title,type\nFavorites,Example,show\n",
	})
	data, err := Parse("bingers", archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Titles) != 1 || data.Titles[0].Status != "dropped" || len(data.Watches) != 1 || data.Watches[0].Episode != 2 || len(data.Ratings) != 1 || data.Ratings[0].Value != 4 || data.Unsupported != 1 || len(data.Issues) != 1 {
		t.Fatalf("parsed data = %+v", data)
	}
	if _, err := Parse("bingers", []byte("not a ZIP")); err == nil {
		t.Fatal("invalid archive accepted")
	}
	if _, err := Parse("bingers", bytes.Repeat([]byte("x"), MaxUpload+1)); err == nil {
		t.Fatal("oversized archive accepted")
	}
	if _, err := Parse("other", archive); err == nil {
		t.Fatal("unsupported source accepted")
	}
	bad := testArchive(t, map[string]string{"library.csv": "type,title\nshow,Example\n", "watches.csv": "type,title\n"})
	if _, err := Parse("bingers", bad); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing header error = %v", err)
	}
}
