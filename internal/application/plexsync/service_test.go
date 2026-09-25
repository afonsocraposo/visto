package plexsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
)

type fakeRepository struct {
	issuedHash string
	userID     string
	logged     []Event
	recorded   []Event
	record     bool
}

func (repository *fakeRepository) IssuePlexWebhook(_ context.Context, _, hash string, _ time.Time) error {
	repository.issuedHash = hash
	return nil
}
func (*fakeRepository) RevokePlexWebhook(context.Context, string) error { return nil }
func (*fakeRepository) GetPlexWebhookStatus(context.Context, string) (Status, error) {
	return Status{}, nil
}
func (repository *fakeRepository) UserForPlexWebhook(context.Context, string, time.Time) (string, error) {
	if repository.userID == "" {
		return "", ErrWebhookNotFound
	}
	return repository.userID, nil
}
func (repository *fakeRepository) LogPlexEvent(_ context.Context, _, _ string, event Event) error {
	repository.logged = append(repository.logged, event)
	return nil
}
func (repository *fakeRepository) RecordPlexPlay(_ context.Context, _, _ string, event Event, _, _ *string, _ time.Time, _ time.Duration) (bool, error) {
	repository.recorded = append(repository.recorded, event)
	return repository.record, nil
}

type fakeMetadataProvider struct {
	searchResults []domain.MediaSearchResult
	movie         domain.MovieMetadata
	show          domain.TVShowMetadata
	season        domain.TVSeasonMetadata
	seasonNumbers []int
}

func (provider *fakeMetadataProvider) Search(context.Context, string, string) ([]domain.MediaSearchResult, error) {
	return provider.searchResults, nil
}
func (provider *fakeMetadataProvider) Movie(_ context.Context, id int64) (domain.MovieMetadata, error) {
	if provider.movie.TMDBID != id {
		return domain.MovieMetadata{}, errors.New("not found")
	}
	return provider.movie, nil
}
func (provider *fakeMetadataProvider) ShowSummary(_ context.Context, id int64) (domain.TVShowMetadata, error) {
	if provider.show.TMDBID != id {
		return domain.TVShowMetadata{}, errors.New("not found")
	}
	return provider.show, nil
}
func (provider *fakeMetadataProvider) Season(_ context.Context, _ int64, number int) (domain.TVSeasonMetadata, error) {
	provider.seasonNumbers = append(provider.seasonNumbers, number)
	if provider.season.Number != number {
		return domain.TVSeasonMetadata{}, errors.New("not found")
	}
	return provider.season, nil
}

type fakeLibrary struct {
	saved        []library.Media
	importedShow domain.TVShowMetadata
	imported     domain.TVSeasonMetadata
}

func (catalog *fakeLibrary) SaveMedia(_ context.Context, _ string, media library.Media, _ domain.LibraryStatus, _ *int) (library.Item, error) {
	catalog.saved = append(catalog.saved, media)
	return library.Item{}, nil
}
func (catalog *fakeLibrary) ImportShowSeason(_ context.Context, _ string, show domain.TVShowMetadata, season domain.TVSeasonMetadata) error {
	catalog.importedShow, catalog.imported = show, season
	return nil
}

func newTestService(t *testing.T, metadata *fakeMetadataProvider) (*Service, *fakeRepository, *fakeLibrary) {
	t.Helper()
	repository := &fakeRepository{userID: "user-1", record: true}
	catalog := &fakeLibrary{}
	service := NewService(repository, metadata, catalog, "https://visto.example.com")
	service.now = func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) }
	return service, repository, catalog
}

func TestHandle_GivenTMDBMovieGUID_WhenPlexScrobblesMovie_ThenItAddsAndRecordsTheMovie(t *testing.T) {
	metadata := &fakeMetadataProvider{movie: domain.MovieMetadata{TMDBID: 10, Title: "Example Movie", OriginalTitle: "Example Movie"}}
	service, repository, catalog := newTestService(t, metadata)
	payload := `{"event":"media.scrobble","Server":{"uuid":"server"},"Player":{"uuid":"player"},"Metadata":{"type":"movie","title":"Example Movie","year":2020,"guid":"tmdb://10","lastViewedAt":1790337600}}`
	if err := service.Handle(context.Background(), "secret", payload); err != nil {
		t.Fatal(err)
	}
	if len(catalog.saved) != 1 || catalog.saved[0].ID != "movie:10" {
		t.Fatalf("saved media = %#v, want movie:10", catalog.saved)
	}
	if len(repository.recorded) != 1 || repository.recorded[0].TMDBID != 10 || repository.recorded[0].Status != "synced" {
		t.Fatalf("recorded events = %#v, want one synced movie event", repository.recorded)
	}
	if got := repository.recorded[0].OccurredAt; !got.Equal(time.Unix(1790337600, 0).UTC()) {
		t.Fatalf("watched timestamp = %s, want Plex timestamp", got)
	}
}

func TestHandle_GivenEpisodeWithoutTMDBGUID_WhenAnExactTVMatchExists_ThenItImportsOnlyThatSeason(t *testing.T) {
	metadata := &fakeMetadataProvider{
		searchResults: []domain.MediaSearchResult{{TMDBID: 42, Type: domain.TVMediaType, Title: "Example Show", OriginalTitle: "Example Show", ReleaseDate: "2020-01-01"}},
		show:          domain.TVShowMetadata{TMDBID: 42, Name: "Example Show", Seasons: []domain.TVSeasonMetadata{{Number: 0}, {Number: 1}, {Number: 2}}},
		season:        domain.TVSeasonMetadata{Number: 2, Episodes: []domain.TVEpisodeMetadata{{TMDBID: 4205, SeasonNumber: 2, EpisodeNumber: 5, Name: "Five"}}},
	}
	service, repository, catalog := newTestService(t, metadata)
	payload := `{"event":"media.scrobble","Metadata":{"type":"episode","title":"Five","grandparentTitle":"Example Show","grandparentYear":2020,"grandparentGuid":"plex://show/abc","parentIndex":2,"index":5}}`
	if err := service.Handle(context.Background(), "secret", payload); err != nil {
		t.Fatal(err)
	}
	if len(metadata.seasonNumbers) != 1 || metadata.seasonNumbers[0] != 2 {
		t.Fatalf("requested seasons = %v, want only season 2", metadata.seasonNumbers)
	}
	if catalog.imported.Number != 2 {
		t.Fatalf("imported season = %d, want season 2", catalog.imported.Number)
	}
	if len(repository.recorded) != 1 || repository.recorded[0].TMDBID != 42 {
		t.Fatalf("recorded events = %#v, want the show event", repository.recorded)
	}
}

func TestHandle_GivenAmbiguousSearchResults_WhenPlexTitleHasNoTMDBGUID_ThenItSkipsWithoutRecording(t *testing.T) {
	metadata := &fakeMetadataProvider{searchResults: []domain.MediaSearchResult{
		{TMDBID: 10, Type: domain.MovieMediaType, Title: "Example Movie", ReleaseDate: "2020-01-01"},
		{TMDBID: 11, Type: domain.MovieMediaType, Title: "Example Movie", ReleaseDate: "2020-01-01"},
	}}
	service, repository, _ := newTestService(t, metadata)
	payload := `{"event":"media.scrobble","Metadata":{"type":"movie","title":"Example Movie","year":2020,"guid":"plex://movie/abc"}}`
	if err := service.Handle(context.Background(), "secret", payload); err != nil {
		t.Fatal(err)
	}
	if len(repository.recorded) != 0 || len(repository.logged) != 1 || repository.logged[0].Status != "skipped" {
		t.Fatalf("recorded=%d logged=%#v; expected one skipped event and no play", len(repository.recorded), repository.logged)
	}
}

func TestHandle_GivenPlaybackProgressEvent_WhenReceived_ThenItIsSafelySkipped(t *testing.T) {
	service, repository, _ := newTestService(t, &fakeMetadataProvider{})
	if err := service.Handle(context.Background(), "secret", `{"event":"media.play","Metadata":{"type":"movie","title":"Example"}}`); err != nil {
		t.Fatal(err)
	}
	if len(repository.logged) != 1 || repository.logged[0].Status != "skipped" || len(repository.recorded) != 0 {
		t.Fatalf("logged=%#v recorded=%d; expected a skipped event", repository.logged, len(repository.recorded))
	}
}

func TestIssue_GivenPublicOrigin_WhenIssuingWebhook_ThenItStoresOnlyAHashAndReturnsASecretURL(t *testing.T) {
	service, repository, _ := newTestService(t, &fakeMetadataProvider{})
	issued, err := service.Issue(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(issued) < len("https://visto.example.com/api/v1/webhooks/plex/")+40 || repository.issuedHash == "" {
		t.Fatal("expected a complete webhook URL and a stored hash")
	}
	if repository.issuedHash == issued {
		t.Fatal("repository stored the raw webhook URL")
	}
}

func TestIssue_GivenMissingOrUnsafePublicURL_WhenCreatingWebhook_ThenItDoesNotIssueASecret(t *testing.T) {
	for _, publicURL := range []string{"", "http://visto.example.com", "https://visto.example.com/path"} {
		t.Run(publicURL, func(t *testing.T) {
			repository := &fakeRepository{}
			service := NewService(repository, nil, nil, publicURL)
			if _, err := service.Issue(context.Background(), "user-1"); err == nil {
				t.Fatal("expected an invalid public URL error")
			}
			if repository.issuedHash != "" {
				t.Fatal("issued a secret for an invalid public URL")
			}
		})
	}
}

func TestSearchID_GivenTitleAndYear_WhenOneExactResultExists_ThenItReturnsTheUniqueTMDBID(t *testing.T) {
	service := &Service{metadata: &fakeMetadataProvider{searchResults: []domain.MediaSearchResult{
		{TMDBID: 10, Type: domain.MovieMediaType, Title: "A Movie!", ReleaseDate: "2020-05-01"},
		{TMDBID: 11, Type: domain.TVMediaType, Title: "A Movie!", ReleaseDate: "2020-05-01"},
	}}}
	id, err := service.searchID(context.Background(), domain.MovieMediaType, "A Movie", "", 2020)
	if err != nil || id != 10 {
		t.Fatalf("TMDB ID = %d, error = %v; want 10 without error", id, err)
	}
}
