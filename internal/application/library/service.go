package library

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

var ErrMediaNotFound = errors.New("media not found in user's library")

type Item struct {
	UserID               string               `json:"user_id"`
	MediaID              string               `json:"media_id"`
	Status               domain.LibraryStatus `json:"status"`
	Rating               *int                 `json:"rating"`
	NotificationsEnabled bool                 `json:"notifications_enabled"`
	AddedAt              time.Time            `json:"added_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
	CreatedEpisodeIDs    []string             `json:"created_episode_ids,omitempty"`
}
type Repository interface {
	UpsertItem(context.Context, Item) error
}

// Media is the small, durable metadata snapshot needed to show a library item.
// Its ID is deterministic so the same TMDB item is shared by every account.
type Media struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	OriginalTitle    string           `json:"original_title"`
	Overview         string           `json:"overview"`
	ReleaseDate      string           `json:"release_date"`
	PosterPath       string           `json:"poster_path"`
	BackdropPath     string           `json:"backdrop_path,omitempty"`
	OriginalLanguage string           `json:"original_language"`
	Status           string           `json:"status,omitempty"`
	Type             domain.MediaType `json:"type"`
	TMDBID           int64            `json:"tmdb_id"`
}

type Entry struct {
	Item          Item                  `json:"item"`
	Media         Media                 `json:"media"`
	Progress      *ShowProgress         `json:"progress,omitempty"`
	MetadataReady bool                  `json:"metadata_ready"`
	Cast          []domain.TVCastMember `json:"cast,omitempty"`
	Runtime       int                   `json:"runtime,omitempty"`
	Genres        []string              `json:"genres,omitempty"`
	VoteAverage   float32               `json:"vote_average,omitempty"`
	DetailsReady  bool                  `json:"-"`
	CatalogReady  bool                  `json:"-"`
}

type ShowProgress struct {
	WatchedEpisodes int `json:"watched_episodes"`
	TotalEpisodes   int `json:"total_episodes"`
}

type mediaRepository interface {
	UpsertMedia(context.Context, Media) error
}

type listRepository interface {
	ListItems(context.Context, string) ([]Entry, error)
}

type sortedListRepository interface {
	ListItemsSorted(context.Context, string, string) ([]Entry, error)
}

type getRepository interface {
	GetMediaByTMDBID(context.Context, string, domain.MediaType, int64) (Entry, error)
}
type watchlistRemovalRepository interface {
	RemoveWatchlistItem(context.Context, string, string) error
}

type statusRemovalRepository interface {
	RemoveStatusItem(context.Context, string, string, domain.LibraryStatus) error
}
type watchingRemovalRepository interface {
	RemoveWatchingItem(context.Context, string, string) error
}
type showMetadataRepository interface {
	ImportShowMetadata(context.Context, string, domain.TVShowMetadata) error
	ShowMetadataNeedsRefresh(context.Context, string, time.Duration) (bool, error)
}
type Service struct {
	repository Repository
	now        func() time.Time
}

type notificationPreferenceRepository interface {
	SetNotificationsEnabled(context.Context, string, string, bool) error
}

type notificationPreferenceReader interface {
	GetNotificationsEnabled(context.Context, string, string) (bool, error)
}

type itemStatusReader interface {
	ItemStatus(context.Context, string, string) (domain.LibraryStatus, error)
}

type completionRepository interface {
	CompleteMedia(context.Context, string, string, *int, string) (Item, error)
}

func (s *Service) Complete(ctx context.Context, userID, mediaID string, rating *int, source string) (Item, error) {
	if userID == "" || mediaID == "" {
		return Item{}, fmt.Errorf("user and media are required")
	}
	repo, ok := s.repository.(completionRepository)
	if !ok {
		return Item{}, fmt.Errorf("completion is not configured")
	}
	return repo.CompleteMedia(ctx, userID, mediaID, rating, source)
}

func (s *Service) StoreMedia(ctx context.Context, media Media) error {
	if media.Type != domain.MovieMediaType && media.Type != domain.TVMediaType || media.TMDBID <= 0 || media.Title == "" {
		return fmt.Errorf("valid media is required")
	}
	media.ID = fmt.Sprintf("%s:%d", media.Type, media.TMDBID)
	repo, ok := s.repository.(mediaRepository)
	if !ok {
		return fmt.Errorf("media storage is not configured")
	}
	return repo.UpsertMedia(ctx, media)
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}
func (s *Service) Save(ctx context.Context, userID, mediaID string, status domain.LibraryStatus, rating *int) (Item, error) {
	if userID == "" || mediaID == "" {
		return Item{}, fmt.Errorf("user and media are required")
	}
	if status != domain.WatchlistStatus && status != domain.WatchingStatus && status != domain.PausedStatus && status != domain.DroppedStatus && status != domain.CompletedStatus {
		return Item{}, fmt.Errorf("invalid library status")
	}
	if strings.HasPrefix(mediaID, "movie:") && status != domain.WatchlistStatus && status != domain.CompletedStatus {
		return Item{}, fmt.Errorf("movies can only be in Watchlist or Completed")
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return Item{}, fmt.Errorf("rating must be from 1 to 5")
	}
	now := s.now().UTC()
	item := Item{UserID: userID, MediaID: mediaID, Status: status, Rating: rating, NotificationsEnabled: true, AddedAt: now, UpdatedAt: now}
	if err := s.repository.UpsertItem(ctx, item); err != nil {
		return Item{}, err
	}
	if reader, ok := s.repository.(itemStatusReader); ok {
		stored, err := reader.ItemStatus(ctx, userID, mediaID)
		if err != nil {
			return Item{}, err
		}
		item.Status = stored
	}
	if reader, ok := s.repository.(notificationPreferenceReader); ok {
		enabled, err := reader.GetNotificationsEnabled(ctx, userID, mediaID)
		if err != nil {
			return Item{}, err
		}
		item.NotificationsEnabled = enabled
	}
	return item, nil
}

func (s *Service) SaveMedia(ctx context.Context, userID string, media Media, status domain.LibraryStatus, rating *int) (Item, error) {
	if media.Type != domain.MovieMediaType && media.Type != domain.TVMediaType {
		return Item{}, fmt.Errorf("invalid media type")
	}
	if media.TMDBID <= 0 || media.Title == "" {
		return Item{}, fmt.Errorf("media TMDB ID and title are required")
	}
	media.ID = fmt.Sprintf("%s:%d", media.Type, media.TMDBID)
	repository, ok := s.repository.(mediaRepository)
	if !ok {
		return Item{}, fmt.Errorf("media storage is not configured")
	}
	if err := repository.UpsertMedia(ctx, media); err != nil {
		return Item{}, err
	}
	return s.Save(ctx, userID, media.ID, status, rating)
}

func (s *Service) List(ctx context.Context, userID string) ([]Entry, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	repository, ok := s.repository.(listRepository)
	if !ok {
		return nil, fmt.Errorf("library storage is not configured")
	}
	return repository.ListItems(ctx, userID)
}

func (s *Service) ListSorted(ctx context.Context, userID, sort string) ([]Entry, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	if sort != "updated" && sort != "title" && sort != "released" {
		return nil, fmt.Errorf("invalid library sort")
	}
	repository, ok := s.repository.(sortedListRepository)
	if !ok {
		return nil, fmt.Errorf("library storage is not configured")
	}
	return repository.ListItemsSorted(ctx, userID, sort)
}

func (s *Service) RemoveWatchlistItem(ctx context.Context, userID, mediaID string) error {
	if userID == "" || mediaID == "" {
		return fmt.Errorf("user and media are required")
	}
	repository, ok := s.repository.(watchlistRemovalRepository)
	if !ok {
		return fmt.Errorf("watchlist removal is not configured")
	}
	return repository.RemoveWatchlistItem(ctx, userID, mediaID)
}

func (s *Service) RemoveStatusItem(ctx context.Context, userID, mediaID string, status domain.LibraryStatus) error {
	if userID == "" || mediaID == "" {
		return fmt.Errorf("user and media are required")
	}
	if status != domain.PausedStatus && status != domain.DroppedStatus {
		return fmt.Errorf("invalid removable status")
	}
	repository, ok := s.repository.(statusRemovalRepository)
	if !ok {
		return fmt.Errorf("library removal is not configured")
	}
	return repository.RemoveStatusItem(ctx, userID, mediaID, status)
}

func (s *Service) RemoveWatchingItem(ctx context.Context, userID, mediaID string) error {
	if userID == "" || mediaID == "" {
		return fmt.Errorf("user and media are required")
	}
	repository, ok := s.repository.(watchingRemovalRepository)
	if !ok {
		return fmt.Errorf("watching removal is not configured")
	}
	return repository.RemoveWatchingItem(ctx, userID, mediaID)
}

func (s *Service) SetNotificationsEnabled(ctx context.Context, userID, mediaID string, enabled bool) error {
	if userID == "" || mediaID == "" {
		return fmt.Errorf("user and media are required")
	}
	if !strings.HasPrefix(mediaID, "tv:") {
		return fmt.Errorf("episode notifications are only available for TV shows")
	}
	repository, ok := s.repository.(notificationPreferenceRepository)
	if !ok {
		return fmt.Errorf("notification preferences are not configured")
	}
	return repository.SetNotificationsEnabled(ctx, userID, mediaID, enabled)
}

func (s *Service) GetByTMDBID(ctx context.Context, userID string, mediaType domain.MediaType, tmdbID int64) (Entry, error) {
	if userID == "" || tmdbID <= 0 {
		return Entry{}, fmt.Errorf("user and a valid TMDB ID are required")
	}
	if mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType {
		return Entry{}, fmt.Errorf("invalid media type")
	}
	repository, ok := s.repository.(getRepository)
	if !ok {
		return Entry{}, fmt.Errorf("library lookup is not configured")
	}
	return repository.GetMediaByTMDBID(ctx, userID, mediaType, tmdbID)
}

// HydrateMissing saves a summary only for a title already in this user's library.
func (s *Service) HydrateMissing(ctx context.Context, userID string, mediaType domain.MediaType, tmdbID int64, provider domain.MetadataProvider) (Entry, error) {
	entry, err := s.GetByTMDBID(ctx, userID, mediaType, tmdbID)
	if err != nil || (entry.MetadataReady && entry.DetailsReady) || provider == nil {
		return entry, err
	}
	if mediaType == domain.TVMediaType {
		fetcher, ok := provider.(domain.TVShowSummaryProvider)
		writer, writable := s.repository.(interface {
			SaveShowSummary(context.Context, domain.TVShowMetadata) error
		})
		if !ok || !writable {
			return entry, nil
		}
		show, fetchErr := fetcher.ShowSummary(ctx, tmdbID)
		if fetchErr != nil || show.TMDBID != tmdbID || show.Name == "" {
			return entry, nil
		}
		if err = writer.SaveShowSummary(ctx, show); err != nil {
			return entry, err
		}
	} else {
		fetcher, ok := provider.(domain.MovieMetadataProvider)
		writer, writable := s.repository.(interface {
			SaveMovieMetadata(context.Context, domain.MovieMetadata) error
		})
		if !ok || !writable {
			return entry, nil
		}
		movie, fetchErr := fetcher.Movie(ctx, tmdbID)
		if fetchErr != nil || movie.TMDBID != tmdbID || movie.Title == "" {
			return entry, nil
		}
		if err = writer.SaveMovieMetadata(ctx, movie); err != nil {
			return entry, err
		}
	}
	return s.GetByTMDBID(ctx, userID, mediaType, tmdbID)
}

func (s *Service) ImportShow(ctx context.Context, tmdbID int64, provider domain.TVShowMetadataProvider) error {
	return s.importShow(ctx, tmdbID, provider, 24*time.Hour)
}

func (s *Service) RefreshShow(ctx context.Context, tmdbID int64, provider domain.TVShowMetadataProvider) error {
	return s.importShow(ctx, tmdbID, provider, 0)
}

func (s *Service) importShow(ctx context.Context, tmdbID int64, provider domain.TVShowMetadataProvider, ttl time.Duration) error {
	if provider == nil {
		return fmt.Errorf("TV metadata provider is not configured")
	}
	repository, ok := s.repository.(showMetadataRepository)
	if !ok {
		return fmt.Errorf("show metadata storage is not configured")
	}
	needsRefresh := ttl == 0
	if !needsRefresh {
		var err error
		needsRefresh, err = repository.ShowMetadataNeedsRefresh(ctx, fmt.Sprintf("tv:%d", tmdbID), ttl)
		if err != nil {
			return err
		}
	}
	if !needsRefresh {
		return nil
	}
	metadata, err := provider.Show(ctx, tmdbID)
	if err != nil {
		return err
	}
	if metadata.TMDBID != tmdbID || metadata.Name == "" {
		return fmt.Errorf("TMDB returned invalid show metadata")
	}
	return repository.ImportShowMetadata(ctx, fmt.Sprintf("tv:%d", tmdbID), metadata)
}

// ImportShowSeason stores show-level metadata and one fetched season. It is
// used by integrations that need a single watched episode without importing
// every episode in a series.
func (s *Service) ImportShowSeason(ctx context.Context, userID string, show domain.TVShowMetadata, season domain.TVSeasonMetadata) error {
	if userID == "" || show.TMDBID <= 0 || show.Name == "" {
		return fmt.Errorf("user and valid TV show metadata are required")
	}
	if season.Number < 0 || len(season.Episodes) == 0 {
		return fmt.Errorf("a valid TV season with episodes is required")
	}
	repository, ok := s.repository.(showMetadataRepository)
	if !ok {
		return fmt.Errorf("show metadata storage is not configured")
	}
	foundSeason := false
	for index := range show.Seasons {
		if show.Seasons[index].Number == season.Number {
			show.Seasons[index] = season
			foundSeason = true
			break
		}
	}
	if !foundSeason {
		show.Seasons = append(show.Seasons, season)
	}
	return repository.ImportShowMetadata(ctx, fmt.Sprintf("tv:%d", show.TMDBID), show)
}
