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
