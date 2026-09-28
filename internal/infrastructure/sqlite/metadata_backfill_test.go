package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

type metadataFixture struct {
	fullCalls    []int64
	summaryCalls []int64
	movieCalls   []int64
	fail43       bool
}

func (*metadataFixture) Search(context.Context, string, string) ([]domain.MediaSearchResult, error) {
	return nil, nil
}
func (f *metadataFixture) ShowSummary(_ context.Context, id int64) (domain.TVShowMetadata, error) {
	f.summaryCalls = append(f.summaryCalls, id)
	if id == 43 && f.fail43 {
		return domain.TVShowMetadata{}, errors.New("TMDB unavailable")
	}
	return domain.TVShowMetadata{TMDBID: id, Name: "Show", Overview: "Summary", PosterPath: "/show.jpg", BackdropPath: "/back.jpg", Cast: []domain.TVCastMember{{ID: 7, Name: "Actor"}}}, nil
}
func (*metadataFixture) Season(context.Context, int64, int) (domain.TVSeasonMetadata, error) {
	return domain.TVSeasonMetadata{}, nil
}
func (f *metadataFixture) Show(_ context.Context, id int64) (domain.TVShowMetadata, error) {
	f.fullCalls = append(f.fullCalls, id)
	if id == 43 && f.fail43 {
		return domain.TVShowMetadata{}, errors.New("TMDB unavailable")
	}
	return domain.TVShowMetadata{TMDBID: id, Name: "Show", Overview: "Summary", PosterPath: "/show.jpg", BackdropPath: "/back.jpg", Cast: []domain.TVCastMember{{ID: 7, Name: "Actor"}}, Seasons: []domain.TVSeasonMetadata{{Number: 1, EpisodeCount: 1, Name: "Season 1", Episodes: []domain.TVEpisodeMetadata{{TMDBID: id*100 + 1, SeasonNumber: 1, EpisodeNumber: 1, Name: "Pilot"}}}}}, nil
}
func (f *metadataFixture) Movie(_ context.Context, id int64) (domain.MovieMetadata, error) {
	f.movieCalls = append(f.movieCalls, id)
	return domain.MovieMetadata{TMDBID: id, Title: "Film", Overview: "Film summary", BackdropPath: "/film.jpg", Runtime: 108, Genres: []string{"Drama"}, VoteAverage: 7.5, Cast: []domain.TVCastMember{{ID: 8, Name: "Film actor"}}}, nil
}

func TestImportedMetadataHydratesOnOpenAndBackfillsWithoutChangingWatches(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner := insertTestUser(t, store.DB, "metadata-owner", "Owner", "private")
	other := insertTestUser(t, store.DB, "metadata-other", "Other", "private")
	for _, item := range []struct {
		kind string
		id   int64
	}{{"movie", 10}, {"tv", 42}, {"tv", 43}, {"tv", 44}} {
		if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES(?,?,?,?, '',?)`, item.kind+":"+itoa(item.id), item.kind, item.id, "Placeholder", testTimestamp); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,?,'watchlist',?,?)`, owner+":"+item.kind+":"+itoa(item.id), owner, item.kind+":"+itoa(item.id), testTimestamp, testTimestamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,name,episode_count) VALUES('tv:42:season:1','tv:42',1,'Season 1',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:42:episode:1:1','tv:42','tv:42:season:1',1,1,'Episode 1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES(?,'tv:42:episode:1:1',?,'import',?)`, owner, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	provider := &metadataFixture{fail43: true}
	service := library.NewService(store)
	show, err := service.HydrateMissing(ctx, owner, domain.TVMediaType, 42, provider)
	if err != nil || !show.MetadataReady || show.CatalogReady || show.Media.PosterPath != "/show.jpg" || len(show.Cast) != 1 {
		t.Fatalf("hydrated show=%+v err=%v", show, err)
	}
	movie, err := service.HydrateMissing(ctx, owner, domain.MovieMediaType, 10, provider)
	if err != nil || !movie.MetadataReady || movie.Runtime != 108 || movie.VoteAverage != 7.5 || len(movie.Genres) != 1 || len(movie.Cast) != 1 || movie.Media.BackdropPath != "/film.jpg" {
		t.Fatalf("hydrated movie=%+v err=%v", movie, err)
	}
	entries, err := store.ListItemsSorted(ctx, owner, "title")
	if err != nil {
		t.Fatal(err)
	}
	var listedMovie bool
	for _, entry := range entries {
		if entry.Media.ID == "movie:10" {
			listedMovie = entry.MetadataReady && entry.Media.BackdropPath == "/film.jpg"
		}
	}
	if !listedMovie {
		t.Fatal("library entry did not expose saved metadata")
	}
	if _, err := service.HydrateMissing(ctx, other, domain.TVMediaType, 42, provider); !errors.Is(err, library.ErrMediaNotFound) {
		t.Fatalf("other user read: %v", err)
	}
	if _, err := service.HydrateMissing(ctx, owner, domain.MovieMediaType, 99, provider); !errors.Is(err, library.ErrMediaNotFound) {
		t.Fatalf("unsaved title read: %v", err)
	}
	if len(provider.summaryCalls) != 1 || len(provider.movieCalls) != 1 {
		t.Fatalf("unexpected fetch calls: %+v", provider)
	}
	if _, err := service.HydrateMissing(ctx, owner, domain.MovieMediaType, 10, provider); err != nil || len(provider.movieCalls) != 1 {
		t.Fatalf("stored detail fetched again: %v", err)
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('movie:11','movie',11,'Old film',?,?)`, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'movie:11','watchlist',?,?)`, owner+":movie:11", owner, testTimestamp, testTimestamp); err != nil {
		t.Fatal(err)
	}
	older, err := service.HydrateMissing(ctx, owner, domain.MovieMediaType, 11, provider)
	if err != nil || !older.DetailsReady || older.Runtime != 108 {
		t.Fatalf("older saved film not filled: %+v err=%v", older, err)
	}
	placeholder, err := service.HydrateMissing(ctx, owner, domain.TVMediaType, 43, provider)
	if err != nil || placeholder.MetadataReady {
		t.Fatalf("failed summary changed placeholder: %+v err=%v", placeholder, err)
	}
	if err := watch.NewService(store, provider).BackfillMissingMetadata(ctx); err != nil {
		t.Fatal(err)
	}
	var episodeID string
	if err := store.DB.QueryRow(`SELECT episode_id FROM plays WHERE user_id=?`, owner).Scan(&episodeID); err != nil {
		t.Fatal(err)
	}
	if episodeID != "tv:42:episode:1:1" {
		t.Fatalf("watch moved to %q", episodeID)
	}
	var tmdbID int64
	if err := store.DB.QueryRow(`SELECT tmdb_id FROM episodes WHERE id=?`, episodeID).Scan(&tmdbID); err != nil || tmdbID != 4201 {
		t.Fatalf("episode metadata id=%d err=%v", tmdbID, err)
	}
	if len(provider.fullCalls) != 3 || provider.fullCalls[0] != 42 || provider.fullCalls[1] != 43 || provider.fullCalls[2] != 44 {
		t.Fatalf("backfill calls=%v", provider.fullCalls)
	}
	ready, err := store.GetMediaByTMDBID(ctx, owner, domain.TVMediaType, 44)
	if err != nil || !ready.MetadataReady || !ready.CatalogReady {
		t.Fatalf("later title not filled: %+v err=%v", ready, err)
	}
	failed, err := store.GetMediaByTMDBID(ctx, owner, domain.TVMediaType, 43)
	if err != nil || failed.MetadataReady {
		t.Fatalf("failed title changed: %+v err=%v", failed, err)
	}
	provider.fail43 = false
	retried, err := service.HydrateMissing(ctx, owner, domain.TVMediaType, 43, provider)
	if err != nil || !retried.MetadataReady {
		t.Fatalf("opening later did not retry: %+v err=%v", retried, err)
	}
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }
