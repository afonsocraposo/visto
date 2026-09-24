package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestImportShowMetadata_GivenShowWithRegularEpisodesAndSpecials_WhenImported_ThenEpisodesAreStoredByTMDBIdentity(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example','2026-09-24T00:00:00Z','2026-09-24T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	err = store.ImportShowMetadata(context.Background(), "tv:42", domain.TVShowMetadata{TMDBID: 42, Name: "Example Show", Seasons: []domain.TVSeasonMetadata{{TMDBID: 420, Number: 0, Name: "Specials", Episodes: []domain.TVEpisodeMetadata{{TMDBID: 4201, SeasonNumber: 0, EpisodeNumber: 1, Name: "Extra"}}}, {TMDBID: 421, Number: 1, Name: "Season 1", Episodes: []domain.TVEpisodeMetadata{{TMDBID: 4211, SeasonNumber: 1, EpisodeNumber: 1, Name: "Pilot", AirDate: "2026-09-20"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	var title string
	if err := store.DB.QueryRow(`SELECT title FROM media WHERE id='tv:42'`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Example Show" {
		t.Fatalf("title=%q", title)
	}
	var regular, special int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM episodes WHERE show_id='tv:42' AND season_number=1 AND tmdb_id=4211`).Scan(&regular); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM episodes WHERE show_id='tv:42' AND season_number=0 AND tmdb_id=4201`).Scan(&special); err != nil {
		t.Fatal(err)
	}
	if regular != 1 || special != 1 {
		t.Fatalf("regular=%d special=%d", regular, special)
	}
}

func TestListShowEpisodes_GivenTwoUsersAndOneWatchedEpisode_WhenRequested_ThenItReturnsOnlyTheOwnersCatalogAndWatchState(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, userID := range []string{"owner", "other"} {
		_, err := store.DB.Exec(`INSERT INTO users(id,username,display_name,password_hash,role,created_at,updated_at) VALUES(?,?,?,'hash','user','2026-09-24T00:00:00Z','2026-09-24T00:00:00Z')`, userID, userID, userID)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Example','2026-09-24T00:00:00Z','2026-09-24T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ImportShowMetadata(context.Background(), "tv:42", domain.TVShowMetadata{
		TMDBID: 42,
		Name:   "Example",
		Seasons: []domain.TVSeasonMetadata{
			{TMDBID: 421, Number: 1, Name: "Season 1", Episodes: []domain.TVEpisodeMetadata{{TMDBID: 4211, SeasonNumber: 1, EpisodeNumber: 1, Name: "Pilot", AirDate: "2026-09-01"}}},
			{TMDBID: 435, Number: 15, Name: "Season 15", Episodes: []domain.TVEpisodeMetadata{{TMDBID: 4351, SeasonNumber: 15, EpisodeNumber: 1, Name: "Premiere", AirDate: "2026-09-15"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES('owner-tv','owner','tv:42','watching','2026-09-24T00:00:00Z','2026-09-24T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO plays(id,user_id,episode_id,watched_at,source,created_at) VALUES('owner-play','owner','tv:42:episode:4211','2026-09-23T00:00:00Z','web','2026-09-23T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := store.ListShowEpisodes(context.Background(), "owner", "tv:42")
	if err != nil {
		t.Fatalf("owner episode list: %v", err)
	}
	if len(entries) != 2 || !entries[0].Watched || entries[1].Watched || entries[1].Episode.SeasonNumber != 15 {
		t.Fatalf("owner episodes=%+v", entries)
	}
	if _, err := store.ListShowEpisodes(context.Background(), "other", "tv:42"); err == nil {
		t.Fatal("expected another user without the show in their library to be denied")
	}
}
