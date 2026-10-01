package plexsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

// A real `media.scrobble` for an episode of a show that Plex titles "The Paper (2025)": the show's
// GUID is a Plex GUID, Metadata.Guid lists the *episode's* external IDs, and no year is sent for
// the show.
const thePaperScrobble = `{"event":"media.scrobble","Account":{"id":123},"Server":{"uuid":"server"},"Player":{"uuid":"player"},
"Metadata":{"type":"episode","title":"Impressing a Cop","grandparentTitle":"The Paper (2025)","parentTitle":"Season 2",
"grandparentGuid":"plex://show/663c5c9154860a808df84899","index":4,"parentIndex":2,"year":2026,"lastViewedAt":1790846007,
"Guid":[{"id":"imdb://tt43347562"},{"id":"tmdb://7423219"},{"id":"tvdb://11861026"}]}}`

const thePaperGUID = "plex://show/663c5c9154860a808df84899"

func thePaperMetadata() *fakeMetadataProvider {
	return &fakeMetadataProvider{
		searchResults: []domain.MediaSearchResult{
			{TMDBID: 7, Type: domain.TVMediaType, Title: "The Paper", OriginalTitle: "The Paper", ReleaseDate: "2019-05-01"},
			{TMDBID: 100, Type: domain.TVMediaType, Title: "The Paper", OriginalTitle: "The Paper", ReleaseDate: "2025-09-04"},
		},
		show:   domain.TVShowMetadata{TMDBID: 100, Name: "The Paper", FirstAirDate: "2025-09-04"},
		season: domain.TVSeasonMetadata{Number: 2, Episodes: []domain.TVEpisodeMetadata{{TMDBID: 9004, SeasonNumber: 2, EpisodeNumber: 4, Name: "Impressing a Cop"}}},
	}
}

// countingProvider records every request that would reach TMDB.
type countingProvider struct {
	*fakeMetadataProvider
	location domain.EpisodeLocation
	findErr  error
	finds    []string
	searches int
	summary  int
}

func (provider *countingProvider) Search(ctx context.Context, query, language string) ([]domain.MediaSearchResult, error) {
	provider.searches++
	return provider.fakeMetadataProvider.Search(ctx, query, language)
}

func (provider *countingProvider) ShowSummary(ctx context.Context, id int64) (domain.TVShowMetadata, error) {
	provider.summary++
	return provider.fakeMetadataProvider.ShowSummary(ctx, id)
}

func (provider *countingProvider) FindEpisodeByExternalID(_ context.Context, source, id string) (domain.EpisodeLocation, error) {
	provider.finds = append(provider.finds, source+":"+id)
	return provider.location, provider.findErr
}

// matchRepository adds the local-knowledge capability to the fake repository.
type matchRepository struct {
	*fakeRepository
	matches    map[string]int64
	local      map[[3]int64]LocalEpisode
	savedGUIDs map[string]int64
	episodeIDs []string
}

func (repository *matchRepository) PlexShowMatch(_ context.Context, guid string) (int64, error) {
	return repository.matches[guid], nil
}

func (repository *matchRepository) SavePlexShowMatch(_ context.Context, guid string, id int64) error {
	if repository.savedGUIDs == nil {
		repository.savedGUIDs = map[string]int64{}
	}
	repository.savedGUIDs[guid] = id
	return nil
}

func (repository *matchRepository) LocalEpisode(_ context.Context, show int64, season, episode int) (LocalEpisode, bool, error) {
	found, ok := repository.local[[3]int64{show, int64(season), int64(episode)}]
	return found, ok, nil
}

func (repository *matchRepository) RecordPlexPlay(ctx context.Context, userID, fingerprint string, event Event, mediaID, episodeID *string, watchedAt time.Time, window time.Duration) (bool, error) {
	if episodeID != nil {
		repository.episodeIDs = append(repository.episodeIDs, *episodeID)
	}
	return repository.fakeRepository.RecordPlexPlay(ctx, userID, fingerprint, event, mediaID, episodeID, watchedAt, window)
}

func newMatchingService(t *testing.T, provider MetadataProvider) (*Service, *matchRepository, *fakeLibrary) {
	t.Helper()
	service, base, catalog := newTestService(t, nil)
	repository := &matchRepository{fakeRepository: base}
	service.repository = repository
	service.metadata = provider
	return service, repository, catalog
}

func TestHandle_GivenShowTitledWithAYear_WhenEpisodeIDsAreKnown_ThenTheEpisodeIDFindsTheShow(t *testing.T) {
	provider := &countingProvider{
		fakeMetadataProvider: thePaperMetadata(),
		location:             domain.EpisodeLocation{ShowTMDBID: 100, SeasonNumber: 2, EpisodeNumber: 4},
	}
	service, repository, catalog := newMatchingService(t, provider)
	if err := service.Handle(context.Background(), "secret", thePaperScrobble); err != nil {
		t.Fatal(err)
	}
	if len(repository.recorded) != 1 || repository.recorded[0].Status != "synced" {
		t.Fatalf("recorded=%#v logged=%#v; want a synced episode", repository.recorded, repository.logged)
	}
	event := repository.recorded[0]
	if event.Title != "The Paper" || event.EpisodeLabel != "S2E4" || event.TMDBID != 100 {
		t.Fatalf("event = %#v, want The Paper S2E4 (tmdb 100)", event)
	}
	if len(provider.finds) != 1 || provider.finds[0] != "tvdb_id:11861026" || provider.searches != 0 {
		t.Fatalf("finds=%v searches=%d; want a single TVDB lookup and no title search", provider.finds, provider.searches)
	}
	if len(catalog.saved) != 1 || catalog.saved[0].ID != "tv:100" {
		t.Fatalf("saved = %#v, want the show stored", catalog.saved)
	}
	if repository.savedGUIDs[thePaperGUID] != 100 {
		t.Fatalf("remembered matches = %v, want the Plex show GUID mapped to TMDB 100", repository.savedGUIDs)
	}
}

func TestHandle_GivenShowTitledWithAYear_WhenOnlyTheTitleIsAvailable_ThenTheYearInTheTitleSelectsTheShow(t *testing.T) {
	service, repository, _ := newTestService(t, thePaperMetadata())
	if err := service.Handle(context.Background(), "secret", thePaperScrobble); err != nil {
		t.Fatal(err)
	}
	if len(repository.recorded) != 1 || repository.recorded[0].TMDBID != 100 {
		t.Fatalf("recorded=%#v logged=%#v; want The Paper (2025), not the 2019 show", repository.recorded, repository.logged)
	}
}

func TestHandle_GivenAnEpisodeIDLookupThatFails_WhenTheTitleMatches_ThenItFallsBackToTheTitleSearch(t *testing.T) {
	provider := &countingProvider{fakeMetadataProvider: thePaperMetadata(), findErr: errors.New("no unique TMDB episode match")}
	service, repository, _ := newMatchingService(t, provider)
	if err := service.Handle(context.Background(), "secret", thePaperScrobble); err != nil {
		t.Fatal(err)
	}
	if len(repository.recorded) != 1 || provider.searches != 1 || len(provider.finds) != 2 {
		t.Fatalf("recorded=%d searches=%d finds=%v; want TVDB then IMDb lookups, then one search", len(repository.recorded), provider.searches, provider.finds)
	}
}

func TestHandle_GivenAShowRememberedFromAnEarlierEvent_WhenTheEpisodeIsInTheCatalog_ThenTMDBIsNotAsked(t *testing.T) {
	provider := &countingProvider{fakeMetadataProvider: thePaperMetadata()}
	service, repository, catalog := newMatchingService(t, provider)
	repository.matches = map[string]int64{thePaperGUID: 100}
	repository.local = map[[3]int64]LocalEpisode{{100, 2, 4}: {EpisodeID: "tv:100:episode:9004", ShowTitle: "The Paper"}}
	if err := service.Handle(context.Background(), "secret", thePaperScrobble); err != nil {
		t.Fatal(err)
	}
	if len(repository.recorded) != 1 || repository.recorded[0].Title != "The Paper" || repository.recorded[0].EpisodeLabel != "S2E4" {
		t.Fatalf("recorded=%#v logged=%#v; want The Paper S2E4", repository.recorded, repository.logged)
	}
	if len(repository.episodeIDs) != 1 || repository.episodeIDs[0] != "tv:100:episode:9004" {
		t.Fatalf("episode IDs = %v, want the catalogued episode", repository.episodeIDs)
	}
	if provider.searches != 0 || len(provider.finds) != 0 || provider.summary != 0 || len(provider.seasonNumbers) != 0 || len(catalog.saved) != 0 {
		t.Fatalf("searches=%d finds=%v summary=%d seasons=%v saved=%d; want no TMDB requests and no re-import", provider.searches, provider.finds, provider.summary, provider.seasonNumbers, len(catalog.saved))
	}
}

func TestHandle_GivenAShowRememberedFromAnEarlierEvent_WhenTheEpisodeIsNew_ThenOnlyItsSeasonIsFetched(t *testing.T) {
	provider := &countingProvider{fakeMetadataProvider: thePaperMetadata()}
	service, repository, catalog := newMatchingService(t, provider)
	repository.matches = map[string]int64{thePaperGUID: 100}
	if err := service.Handle(context.Background(), "secret", thePaperScrobble); err != nil {
		t.Fatal(err)
	}
	if len(repository.recorded) != 1 || provider.searches != 0 || len(provider.finds) != 0 {
		t.Fatalf("recorded=%d searches=%d finds=%v; want the remembered show without a lookup", len(repository.recorded), provider.searches, provider.finds)
	}
	if len(catalog.saved) != 1 || len(provider.seasonNumbers) != 1 {
		t.Fatalf("saved=%d seasons=%v; want the season fetched and stored once", len(catalog.saved), provider.seasonNumbers)
	}
	if len(repository.savedGUIDs) != 0 {
		t.Fatalf("remembered matches = %v; a known show must not be written again", repository.savedGUIDs)
	}
}

func TestHandle_GivenAnUnmatchedEpisode_WhenLogged_ThenTheHistoryNamesTheShowAndEpisodeNumber(t *testing.T) {
	service, repository, _ := newTestService(t, &fakeMetadataProvider{})
	if err := service.Handle(context.Background(), "secret", thePaperScrobble); err != nil {
		t.Fatal(err)
	}
	if len(repository.logged) != 1 {
		t.Fatalf("logged = %#v, want one skipped event", repository.logged)
	}
	event := repository.logged[0]
	if event.Status != "skipped" || event.Title != "The Paper" || event.EpisodeLabel != "S2E4" {
		t.Fatalf("event = %#v, want a skipped event titled The Paper with S2E4, not the episode title", event)
	}
}

func TestHandle_GivenAMovie_WhenLogged_ThenTheHistoryShowsTheMovieTitleWithoutAnEpisodeLabel(t *testing.T) {
	service, repository, _ := newTestService(t, &fakeMetadataProvider{})
	payload := `{"event":"media.scrobble","Account":{"id":123},"Metadata":{"type":"movie","title":"Example Movie","year":2020}}`
	if err := service.Handle(context.Background(), "secret", payload); err != nil {
		t.Fatal(err)
	}
	if len(repository.logged) != 1 || repository.logged[0].Title != "Example Movie" || repository.logged[0].EpisodeLabel != "" {
		t.Fatalf("logged = %#v, want the movie title and no episode label", repository.logged)
	}
}

func TestSplitPlexTitle(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
		year int
	}{
		{"The Paper (2025)", "The Paper", 2025},
		{"  Dune (2021) ", "Dune", 2021},
		{"Blade Runner 2049", "Blade Runner 2049", 0},
		{"Ghosts (US)", "Ghosts (US)", 0},
		{"(2025)", "", 2025},
		{"Show (20255)", "Show (20255)", 0},
	} {
		if got, year := splitPlexTitle(test.in); got != test.want || year != test.year {
			t.Errorf("splitPlexTitle(%q) = %q, %d; want %q, %d", test.in, got, year, test.want, test.year)
		}
	}
}

func TestParseTMDBGUID_ReadsPlainGUIDStrings(t *testing.T) {
	for guid, want := range map[string]int64{
		"tmdb://1396": 1396,
		"com.plexapp.agents.themoviedb://1396?lang=en":    1396,
		"https://www.themoviedb.org/tv/1396-breaking-bad": 1396,
		"plex://show/663c5c9154860a808df84899":            0,
		"":                                                0,
	} {
		if got := parseTMDBGUID(guid); got != want {
			t.Errorf("parseTMDBGUID(%q) = %d, want %d", guid, got, want)
		}
	}
}

func TestExternalEpisodeIDs_PrefersTVDBAndIgnoresOtherIDs(t *testing.T) {
	got := externalEpisodeIDs([]byte(`[{"id":"imdb://tt43347562"},{"id":"tmdb://7423219"},{"id":"tvdb://11861026"},{"id":"plex://x"}]`))
	want := []externalID{{"tvdb_id", "11861026"}, {"imdb_id", "tt43347562"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("externalEpisodeIDs = %v, want %v", got, want)
	}
	if externalEpisodeIDs([]byte(`"not a list"`)) != nil || externalEpisodeIDs(nil) != nil {
		t.Fatal("malformed Guid values must yield no IDs")
	}
}
