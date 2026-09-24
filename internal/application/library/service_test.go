package library_test

import (
	"context"
	"errors"
	"testing"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
)

type repo struct {
	item                   library.Item
	media                  library.Media
	entries                []library.Entry
	notificationPreference struct {
		userID  string
		mediaID string
		enabled bool
	}
	lookup struct {
		userID string
		typeID domain.MediaType
		tmdbID int64
	}
}

func (r *repo) UpsertItem(_ context.Context, item library.Item) error          { r.item = item; return nil }
func (r *repo) UpsertMedia(_ context.Context, media library.Media) error       { r.media = media; return nil }
func (r *repo) ListItems(_ context.Context, _ string) ([]library.Entry, error) { return r.entries, nil }
func (r *repo) GetMediaByTMDBID(_ context.Context, userID string, mediaType domain.MediaType, tmdbID int64) (library.Entry, error) {
	r.lookup.userID, r.lookup.typeID, r.lookup.tmdbID = userID, mediaType, tmdbID
	if len(r.entries) == 0 {
		return library.Entry{}, library.ErrMediaNotFound
	}
	return r.entries[0], nil
}
func (r *repo) SetNotificationsEnabled(_ context.Context, userID, mediaID string, enabled bool) error {
	r.notificationPreference.userID, r.notificationPreference.mediaID, r.notificationPreference.enabled = userID, mediaID, enabled
	return nil
}

func TestGetByTMDBID_GivenAnOwnedTVShow_WhenRequested_ThenItUsesTheAuthenticatedUserAndMediaType(t *testing.T) {
	entry := library.Entry{Item: library.Item{UserID: "owner", MediaID: "tv:42"}, Media: library.Media{ID: "tv:42", Type: domain.TVMediaType, TMDBID: 42, Title: "Example"}}
	repository := &repo{entries: []library.Entry{entry}}
	result, err := library.NewService(repository).GetByTMDBID(context.Background(), "owner", domain.TVMediaType, 42)
	if err != nil {
		t.Fatalf("get show details: %v", err)
	}
	if result.Media.ID != "tv:42" || repository.lookup.userID != "owner" || repository.lookup.typeID != domain.TVMediaType || repository.lookup.tmdbID != 42 {
		t.Fatalf("result=%+v lookup=%+v", result, repository.lookup)
	}
}

func TestGetByTMDBID_GivenUnknownMedia_WhenRequested_ThenItReturnsNotFound(t *testing.T) {
	_, err := library.NewService(&repo{}).GetByTMDBID(context.Background(), "owner", domain.MovieMediaType, 99)
	if !errors.Is(err, library.ErrMediaNotFound) {
		t.Fatalf("error=%v, want not found", err)
	}
}

func TestSave_GivenFiveStarRating_WhenSaving_ThenItPersists(t *testing.T) {
	r := &repo{}
	rating := 5
	item, err := library.NewService(r).Save(context.Background(), "u", "m", domain.WatchingStatus, &rating)
	if err != nil {
		t.Fatal(err)
	}
	if item.Rating == nil || *item.Rating != 5 {
		t.Fatalf("rating=%v", item.Rating)
	}
}

func TestSaveMedia_GivenTMDBMovie_WhenSaving_ThenItUsesStableMediaID(t *testing.T) {
	r := &repo{}
	_, err := library.NewService(r).SaveMedia(context.Background(), "u", library.Media{Type: domain.MovieMediaType, TMDBID: 42, Title: "Answer"}, domain.WatchlistStatus, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.media.ID != "movie:42" || r.item.MediaID != "movie:42" {
		t.Fatalf("media IDs = %q and %q", r.media.ID, r.item.MediaID)
	}
}

func TestSave_GivenMovie_WhenPausedOrDropped_ThenItRejectsTheStatus(t *testing.T) {
	for _, status := range []domain.LibraryStatus{domain.PausedStatus, domain.DroppedStatus} {
		t.Run(string(status), func(t *testing.T) {
			_, err := library.NewService(&repo{}).Save(context.Background(), "u", "movie:42", status, nil)
			if err == nil {
				t.Fatalf("movie status %q was accepted", status)
			}
		})
	}
}

func TestSave_GivenSixStarRating_WhenSaving_ThenItRejectsIt(t *testing.T) {
	rating := 6
	_, err := library.NewService(&repo{}).Save(context.Background(), "u", "m", domain.WatchingStatus, &rating)
	if err == nil {
		t.Fatal("expected rating error")
	}
}

func TestSetNotificationsEnabled_GivenTrackedShow_WhenDisabled_ThenItStoresAnOwnerScopedPreference(t *testing.T) {
	repository := &repo{}
	if err := library.NewService(repository).SetNotificationsEnabled(context.Background(), "owner", "tv:42", false); err != nil {
		t.Fatal(err)
	}
	if repository.notificationPreference.userID != "owner" || repository.notificationPreference.mediaID != "tv:42" || repository.notificationPreference.enabled {
		t.Fatalf("preference=%+v", repository.notificationPreference)
	}
}

func TestSetNotificationsEnabled_GivenMovie_WhenRequested_ThenItRejectsThePreference(t *testing.T) {
	if err := library.NewService(&repo{}).SetNotificationsEnabled(context.Background(), "owner", "movie:42", true); err == nil {
		t.Fatal("expected movie alert preference to be rejected")
	}
}
