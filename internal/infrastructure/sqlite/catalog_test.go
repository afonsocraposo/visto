package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

type repairingCatalogProvider struct {
	seasons []int
	fail    bool
}

func (p *repairingCatalogProvider) Show(ctx context.Context, id int64) (domain.TVShowMetadata, error) {
	return p.RefreshShow(ctx, id)
}

func (p *repairingCatalogProvider) ShowSummary(ctx context.Context, id int64) (domain.TVShowMetadata, error) {
	return p.RefreshShow(ctx, id)
}

func (p *repairingCatalogProvider) RefreshShow(context.Context, int64) (domain.TVShowMetadata, error) {
	return domain.TVShowMetadata{TMDBID: 42, Name: "Show", Status: "Ended", Seasons: []domain.TVSeasonMetadata{
		{Number: 1, EpisodeCount: 2},
		{Number: 2, EpisodeCount: 1, Episodes: []domain.TVEpisodeMetadata{{TMDBID: 201, SeasonNumber: 2, EpisodeNumber: 1}}},
	}}, nil
}

func (p *repairingCatalogProvider) Season(_ context.Context, _ int64, number int) (domain.TVSeasonMetadata, error) {
	p.seasons = append(p.seasons, number)
	if p.fail {
		return domain.TVSeasonMetadata{}, errors.New("season unavailable")
	}
	return domain.TVSeasonMetadata{Number: number, Episodes: []domain.TVEpisodeMetadata{
		{TMDBID: 101, SeasonNumber: number, EpisodeNumber: 1},
		{TMDBID: 102, SeasonNumber: number, EpisodeNumber: 2},
	}}, nil
}

func TestRefreshCatalog_RepairsOldSeasonWithoutLosingHistory(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "viewer", "Viewer", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Show','2026-01-01','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','watching','2026-01-01','2026-01-01')`, userID+":tv:42", userID); err != nil {
		t.Fatal(err)
	}
	if err := store.ImportShowMetadata(ctx, "tv:42", domain.TVShowMetadata{TMDBID: 42, Name: "Show", Status: "Ended", Seasons: []domain.TVSeasonMetadata{
		{Number: 1, Episodes: []domain.TVEpisodeMetadata{{TMDBID: 101, SeasonNumber: 1, EpisodeNumber: 1}, {TMDBID: 103, SeasonNumber: 1, EpisodeNumber: 3}}},
		{Number: 2, Episodes: []domain.TVEpisodeMetadata{{TMDBID: 201, SeasonNumber: 2, EpisodeNumber: 1}}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:42:episode:1:2','tv:42','tv:42:season:1',1,2,'Episode 2')`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"tv:42:episode:101", "tv:42:episode:1:2", "tv:42:episode:103", "tv:42:episode:201"} {
		if _, err := store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,created_at) VALUES(?,?,?,?)`, userID, id, "2026-01-01", "2026-01-01"); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := store.ShowsNeedingCatalogRefresh(ctx, 24*time.Hour, 30*24*time.Hour, 10)
	if err != nil || !slices.Equal(ids, []int64{42}) {
		t.Fatalf("recent ended show eligible for repair: ids=%v err=%v", ids, err)
	}
	var before string
	if err := store.DB.QueryRow(`SELECT catalog_updated_at FROM media WHERE id='tv:42'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	provider := &repairingCatalogProvider{fail: true}
	service := watch.NewService(store, provider)
	if err := service.RefreshCatalog(ctx, 24*time.Hour, 30*24*time.Hour); err == nil {
		t.Fatal("expected season fetch failure")
	}
	var after string
	if err := store.DB.QueryRow(`SELECT catalog_updated_at FROM media WHERE id='tv:42'`).Scan(&after); err != nil || after != before {
		t.Fatalf("failed fetch changed catalog timestamp: before=%q after=%q err=%v", before, after, err)
	}
	provider.fail = false
	provider.seasons = nil
	if err := service.RefreshCatalog(ctx, 24*time.Hour, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(provider.seasons, []int{1}) {
		t.Fatalf("fetched seasons=%v, want only old season 1", provider.seasons)
	}
	var canonicalID int64
	if err := store.DB.QueryRow(`SELECT tmdb_id FROM episodes WHERE id='tv:42:episode:1:2'`).Scan(&canonicalID); err != nil || canonicalID != 102 {
		t.Fatalf("synthetic episode identity=%d err=%v", canonicalID, err)
	}
	var active, plays int
	if err := store.DB.QueryRow(`SELECT active FROM episodes WHERE id='tv:42:episode:103'`).Scan(&active); err != nil || active != 0 {
		t.Fatalf("obsolete episode active=%d err=%v", active, err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE episode_id IN ('tv:42:episode:1:2','tv:42:episode:103')`).Scan(&plays); err != nil || plays != 2 {
		t.Fatalf("preserved plays=%d err=%v", plays, err)
	}
	var status string
	if err := store.DB.QueryRow(`SELECT status FROM user_media WHERE user_id=? AND media_id='tv:42'`, userID).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("show status=%q err=%v", status, err)
	}
}

func TestShowsNeedingCatalogRefresh_GivenManyTrackedShows_WhenLimited_ThenItReturnsOnlyTheBoundedBatch(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	userID := insertTestUser(t, store.DB, "u", "u", "private")
	for id := int64(1); id <= 4; id++ {
		mediaID := fmt.Sprintf("tv:%d", id)
		if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,status,metadata_updated_at,catalog_updated_at,created_at) VALUES(?,'tv',?,?,'Returning','2026-01-01','2026-01-01','2026-01-01')`, mediaID, id, mediaID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?, ?, ?, 'watching', '2026-01-01', '2026-01-01')`, userID+":"+mediaID, userID, mediaID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,status,metadata_updated_at,catalog_updated_at,created_at) VALUES('tv:5','tv',5,'Damaged','Ended',?,?,?)`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), "2026-01-01"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(? ,?,'tv:5','watching','2026-01-01','2026-01-01')`, userID+":tv:5", userID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO seasons(id,show_id,season_number,episode_count) VALUES('tv:5:season:1','tv:5',1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:99','tv',99,'Placeholder','','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:99','watching','2026-01-01','2026-01-01')`, userID+":tv:99", userID); err != nil {
		t.Fatal(err)
	}

	ids, err := store.ShowsNeedingCatalogRefresh(context.Background(), 24*time.Hour, 30*24*time.Hour, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 5 || ids[1] == 99 {
		t.Fatalf("refresh IDs=%v, want damaged show first in a bounded batch", ids)
	}
}

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
	ownerID := insertTestUser(t, store.DB, "owner", "Owner", "private")
	otherID := insertTestUser(t, store.DB, "other", "Other", "private")
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
	_, err = store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(? ,?,'tv:42','watching','2026-09-24T00:00:00Z','2026-09-24T00:00:00Z')`, ownerID+":tv:42", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,source,created_at) VALUES(?,'tv:42:episode:4211','2026-09-23T00:00:00Z','web','2026-09-23T00:00:00Z')`, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	showEntry, err := store.GetMediaByTMDBID(context.Background(), ownerID, domain.TVMediaType, 42)
	if err != nil {
		t.Fatalf("get owner's show details: %v", err)
	}
	if showEntry.Media.Title != "Example" || showEntry.Item.UserID != ownerID {
		t.Fatalf("owner show entry=%+v", showEntry)
	}
	if _, err := store.GetMediaByTMDBID(context.Background(), otherID, domain.TVMediaType, 42); err == nil {
		t.Fatal("expected another user without the show in their library to be denied details access")
	}

	entries, err := store.ListShowEpisodes(context.Background(), ownerID, "tv:42")
	if err != nil {
		t.Fatalf("owner episode list: %v", err)
	}
	if len(entries) != 2 || !entries[0].Watched || entries[1].Watched || entries[1].Episode.SeasonNumber != 15 {
		t.Fatalf("owner episodes=%+v", entries)
	}
	seasons, err := store.ListShowSeasons(context.Background(), ownerID, "tv:42")
	if err != nil {
		t.Fatalf("owner season list: %v", err)
	}
	if len(seasons) != 2 || seasons[1].Number != 15 || seasons[1].EpisodeCount != 1 {
		t.Fatalf("owner seasons=%+v", seasons)
	}
	seasonEpisodes, err := store.ListSeasonEpisodes(context.Background(), ownerID, "tv:42:season:15")
	if err != nil {
		t.Fatalf("owner season episodes: %v", err)
	}
	if len(seasonEpisodes) != 1 || seasonEpisodes[0].Episode.ID != "tv:42:episode:4351" {
		t.Fatalf("season 15 episodes=%+v", seasonEpisodes)
	}
	if _, err := store.ListShowSeasons(context.Background(), otherID, "tv:42"); err == nil {
		t.Fatal("expected another user without the show in their library to be denied season access")
	}
	if _, err := store.ListSeasonEpisodes(context.Background(), otherID, "tv:42:season:15"); err == nil {
		t.Fatal("expected another user without the show in their library to be denied episode access")
	}
	if _, err := store.ListShowEpisodes(context.Background(), otherID, "tv:42"); err == nil {
		t.Fatal("expected another user without the show in their library to be denied")
	}
}

func TestImportShowMetadata_ReconcilesRemovedEpisodesAndKeepsHistory(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "viewer", "Viewer", "private")
	if _, err := store.DB.Exec(`INSERT INTO media(id,media_type,tmdb_id,title,metadata_updated_at,created_at) VALUES('tv:42','tv',42,'Show','','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO user_media(id,user_id,media_id,status,added_at,updated_at) VALUES(?,?,'tv:42','watching','2026-01-01','2026-01-01')`, userID+":tv:42", userID); err != nil {
		t.Fatal(err)
	}
	season := domain.TVSeasonMetadata{Number: 1, EpisodeCount: 4, Episodes: []domain.TVEpisodeMetadata{
		{TMDBID: 101, SeasonNumber: 1, EpisodeNumber: 1, Name: "Released", AirDate: "2020-01-01"},
		{TMDBID: 102, SeasonNumber: 1, EpisodeNumber: 2, Name: "Watched removal", AirDate: "2020-01-01"},
		{TMDBID: 103, SeasonNumber: 1, EpisodeNumber: 3, Name: "Unwatched removal"},
		{TMDBID: 104, SeasonNumber: 1, EpisodeNumber: 4, Name: "Unscheduled"},
	}}
	show := domain.TVShowMetadata{TMDBID: 42, Name: "Show", Seasons: []domain.TVSeasonMetadata{season}}
	if err := store.ImportShowMetadata(ctx, "tv:42", show); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO plays(user_id,episode_id,watched_at,created_at) VALUES(?,'tv:42:episode:102','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO episodes(id,show_id,season_id,season_number,episode_number,name) VALUES('tv:42:episode:1:5','tv:42','tv:42:season:1',1,5,'Episode 5')`); err != nil {
		t.Fatal(err)
	}
	show.Seasons[0].Episodes = []domain.TVEpisodeMetadata{season.Episodes[0], season.Episodes[3]}
	if err := store.ImportShowMetadata(ctx, "tv:42", show); err != nil {
		t.Fatal(err)
	}
	var count, active int
	if err := store.DB.QueryRow(`SELECT episode_count FROM seasons WHERE id='tv:42:season:1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("episode count=%d, want 2", count)
	}
	if err := store.DB.QueryRow(`SELECT active FROM episodes WHERE id='tv:42:episode:102'`).Scan(&active); err != nil || active != 0 {
		t.Fatalf("watched removal active=%d, err=%v", active, err)
	}
	var plays int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM plays WHERE episode_id='tv:42:episode:102'`).Scan(&plays); err != nil || plays != 1 {
		t.Fatalf("history=%d, err=%v", plays, err)
	}
	history, err := store.ListPlays(ctx, userID, 10)
	if err != nil || len(history) != 1 || history[0].EpisodeLabel != "S01E02" {
		t.Fatalf("removed episode history=%v, err=%v", history, err)
	}
	for _, id := range []string{"tv:42:episode:103", "tv:42:episode:1:5"} {
		var exists bool
		if err := store.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM episodes WHERE id=?)`, id).Scan(&exists); err != nil || exists {
			t.Fatalf("obsolete %s still exists: %v", id, err)
		}
	}
	entries, err := store.ListShowEpisodes(ctx, userID, "tv:42")
	if err != nil || len(entries) != 2 {
		t.Fatalf("active entries=%v, err=%v", entries, err)
	}
	unscheduled := "tv:42:episode:104"
	if _, err := store.CreatePlay(ctx, tracking.Play{UserID: userID, EpisodeID: &unscheduled, WatchedAt: time.Now().UTC(), Source: "web"}); err != nil {
		t.Fatalf("manually watch undated episode: %v", err)
	}
	show.Seasons[0].Episodes = append(show.Seasons[0].Episodes, domain.TVEpisodeMetadata{TMDBID: 105, SeasonNumber: 1, EpisodeNumber: 5, Name: "New", AirDate: "2030-01-01"})
	if err := store.ImportShowMetadata(ctx, "tv:42", show); err != nil {
		t.Fatal(err)
	}
	entries, err = store.ListShowEpisodes(ctx, userID, "tv:42")
	if err != nil || len(entries) != 3 || entries[2].Episode.ID != "tv:42:episode:105" {
		t.Fatalf("new episode=%v, err=%v", entries, err)
	}
}

type fixedShowProvider struct{ show domain.TVShowMetadata }

func (provider fixedShowProvider) Show(context.Context, int64) (domain.TVShowMetadata, error) {
	return provider.show, nil
}

func TestRefreshShow_GivenSavedShowWithPartialCast_WhenRefreshed_ThenTheAggregateCastReplacesIt(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	userID := insertTestUser(t, store.DB, "u", "u", "private")
	libraries := library.NewService(store)
	if _, err := libraries.SaveMedia(ctx, userID, library.Media{Type: domain.TVMediaType, TMDBID: 2316, Title: "The Office"}, domain.WatchingStatus, nil); err != nil {
		t.Fatal(err)
	}
	show := domain.TVShowMetadata{TMDBID: 2316, Name: "The Office", Cast: []domain.TVCastMember{{ID: 99, Name: "Final Season Only", Character: "Newcomer"}}}
	if err := libraries.RefreshShow(ctx, 2316, fixedShowProvider{show}); err != nil {
		t.Fatal(err)
	}
	show.Cast = []domain.TVCastMember{
		{ID: 1, Name: "Steve Carell", Character: "Michael Scott", ProfilePath: "/steve.jpg"},
		{ID: 2, Name: "Rainn Wilson", Character: "Dwight Schrute"},
		{ID: 3, Name: "John Krasinski", Character: "Jim Halpert"},
	}
	if err := libraries.RefreshShow(ctx, 2316, fixedShowProvider{show}); err != nil {
		t.Fatal(err)
	}
	entry, err := libraries.GetByTMDBID(ctx, userID, domain.TVMediaType, 2316)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(entry.Cast, show.Cast) {
		t.Fatalf("saved cast=%+v, want refreshed aggregate cast %+v", entry.Cast, show.Cast)
	}
}
