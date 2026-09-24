package library

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

var ErrMediaNotFound = errors.New("media not found in user's library")

type Item struct {
	UserID    string               `json:"user_id"`
	MediaID   string               `json:"media_id"`
	Status    domain.LibraryStatus `json:"status"`
	Rating    *int                 `json:"rating"`
	AddedAt   time.Time            `json:"added_at"`
	UpdatedAt time.Time            `json:"updated_at"`
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
	OriginalLanguage string           `json:"original_language"`
	Type             domain.MediaType `json:"type"`
	TMDBID           int64            `json:"tmdb_id"`
}

type Entry struct {
	Item  Item  `json:"item"`
	Media Media `json:"media"`
}

type mediaRepository interface {
	UpsertMedia(context.Context, Media) error
}

type listRepository interface {
	ListItems(context.Context, string) ([]Entry, error)
}

type getRepository interface {
	GetMediaByTMDBID(context.Context, string, domain.MediaType, int64) (Entry, error)
}
type showMetadataRepository interface {
	ImportShowMetadata(context.Context, string, domain.TVShowMetadata) error
	ShowMetadataNeedsRefresh(context.Context, string, time.Duration) (bool, error)
}
type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}
func (s *Service) Save(ctx context.Context, userID, mediaID string, status domain.LibraryStatus, rating *int) (Item, error) {
	if userID == "" || mediaID == "" {
		return Item{}, fmt.Errorf("user and media are required")
	}
	if status != domain.WatchlistStatus && status != domain.WatchingStatus && status != domain.PausedStatus && status != domain.DroppedStatus {
		return Item{}, fmt.Errorf("invalid library status")
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return Item{}, fmt.Errorf("rating must be from 1 to 5")
	}
	now := s.now().UTC()
	item := Item{UserID: userID, MediaID: mediaID, Status: status, Rating: rating, AddedAt: now, UpdatedAt: now}
	if err := s.repository.UpsertItem(ctx, item); err != nil {
		return Item{}, err
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

func (s *Service) ImportShow(ctx context.Context, tmdbID int64, provider domain.TVShowMetadataProvider) error {
	if provider == nil {
		return fmt.Errorf("TV metadata provider is not configured")
	}
	repository, ok := s.repository.(showMetadataRepository)
	if !ok {
		return fmt.Errorf("show metadata storage is not configured")
	}
	needsRefresh, err := repository.ShowMetadataNeedsRefresh(ctx, fmt.Sprintf("tv:%d", tmdbID), 24*time.Hour)
	if err != nil {
		return err
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
