package tracking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrFutureWatchTime = errors.New("watched_at cannot be in the future")
	ErrPlayNotFound    = errors.New("play not found")
)

type Play struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	MediaID   *string   `json:"media_id"`
	EpisodeID *string   `json:"episode_id"`
	WatchedAt time.Time `json:"watched_at"`
	Source    string    `json:"source"`
}

type HistoryEntry struct {
	Play         Play   `json:"play"`
	Title        string `json:"title"`
	EpisodeLabel string `json:"episode_label,omitempty"`
	EpisodeName  string `json:"episode_name,omitempty"`
	ArtworkPath  string `json:"artwork_path,omitempty"`
}

type EpisodeRating struct {
	EpisodeID string    `json:"episode_id"`
	Rating    *int      `json:"rating"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Repository interface {
	CreatePlay(context.Context, Play) (Play, error)
	CreateBulkPlays(context.Context, []Play) ([]Play, error)
	UpdatePlay(context.Context, string, string, time.Time) error
	DeletePlay(context.Context, string, string) error
}

type historyRepository interface {
	ListPlays(context.Context, string, int) ([]HistoryEntry, error)
}

type episodePlayDeletionRepository interface {
	DeleteEpisodePlays(context.Context, string, []string) error
}

type mediaPlayDeletionRepository interface {
	DeleteMediaPlays(context.Context, string, string) error
}

type episodeRatingRepository interface {
	GetEpisodeRating(context.Context, string, string) (EpisodeRating, error)
	SaveEpisodeRating(context.Context, string, string, *int) (EpisodeRating, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (service *Service) Record(ctx context.Context, userID string, mediaID, episodeID *string, watchedAt time.Time, source string) (Play, error) {
	if userID == "" {
		return Play{}, fmt.Errorf("user is required")
	}
	if (mediaID == nil && episodeID == nil) || (mediaID != nil && episodeID != nil) {
		return Play{}, fmt.Errorf("a play must identify exactly one movie or episode")
	}
	now := service.now().UTC()
	if watchedAt.IsZero() {
		watchedAt = now
	}
	if watchedAt.After(now) {
		return Play{}, ErrFutureWatchTime
	}
	if source == "" {
		source = "web"
	}
	if !validSource(source) {
		return Play{}, fmt.Errorf("invalid play source")
	}
	play := Play{UserID: userID, MediaID: mediaID, EpisodeID: episodeID, WatchedAt: watchedAt.UTC(), Source: source}
	play, err := service.repository.CreatePlay(ctx, play)
	if err != nil {
		return Play{}, err
	}
	return play, nil
}

func (service *Service) RecordEpisodes(ctx context.Context, userID string, episodeIDs []string, watchedAt time.Time, source string) ([]Play, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	if len(episodeIDs) == 0 || len(episodeIDs) > 100 {
		return nil, fmt.Errorf("bulk action must include 1 to 100 episodes")
	}
	now := service.now().UTC()
	if watchedAt.IsZero() {
		watchedAt = now
	}
	if watchedAt.After(now) {
		return nil, ErrFutureWatchTime
	}
	if source == "" {
		source = "web"
	}
	if !validSource(source) {
		return nil, fmt.Errorf("invalid play source")
	}
	seen := map[string]bool{}
	plays := make([]Play, 0, len(episodeIDs))
	for _, episodeID := range episodeIDs {
		if episodeID == "" || seen[episodeID] {
			return nil, fmt.Errorf("episode IDs must be non-empty and unique")
		}
		seen[episodeID] = true
		id := episodeID
		plays = append(plays, Play{UserID: userID, EpisodeID: &id, WatchedAt: watchedAt.UTC(), Source: source})
	}
	plays, err := service.repository.CreateBulkPlays(ctx, plays)
	if err != nil {
		return nil, err
	}
	return plays, nil
}

func validSource(source string) bool {
	switch source {
	case "web", "api", "mcp", "import", "plex":
		return true
	default:
		return false
	}
}

func (service *Service) Correct(ctx context.Context, userID, playID string, watchedAt time.Time) error {
	if userID == "" || playID == "" {
		return fmt.Errorf("user and play are required")
	}
	if watchedAt.IsZero() {
		return fmt.Errorf("watched_at is required")
	}
	if watchedAt.After(service.now().UTC()) {
		return ErrFutureWatchTime
	}
	if err := service.repository.UpdatePlay(ctx, userID, playID, watchedAt.UTC()); err != nil {
		return err
	}
	return nil
}

func (service *Service) Remove(ctx context.Context, userID, playID string) error {
	if userID == "" || playID == "" {
		return fmt.Errorf("user and play are required")
	}
	return service.repository.DeletePlay(ctx, userID, playID)
}

func (service *Service) RemoveEpisodes(ctx context.Context, userID string, episodeIDs []string) error {
	if userID == "" {
		return fmt.Errorf("user is required")
	}
	if len(episodeIDs) == 0 || len(episodeIDs) > 100 {
		return fmt.Errorf("bulk action must include 1 to 100 episodes")
	}
	seen := map[string]bool{}
	for _, episodeID := range episodeIDs {
		if episodeID == "" || seen[episodeID] {
			return fmt.Errorf("episode IDs must be non-empty and unique")
		}
		seen[episodeID] = true
	}
	repository, ok := service.repository.(episodePlayDeletionRepository)
	if !ok {
		return fmt.Errorf("bulk episode deletion is not configured")
	}
	return repository.DeleteEpisodePlays(ctx, userID, episodeIDs)
}

func (service *Service) RemoveMediaPlays(ctx context.Context, userID, mediaID string) error {
	if userID == "" || mediaID == "" {
		return fmt.Errorf("user and media are required")
	}
	if !strings.HasPrefix(mediaID, "movie:") {
		return fmt.Errorf("only movie watch history can be removed by media ID")
	}
	repository, ok := service.repository.(mediaPlayDeletionRepository)
	if !ok {
		return fmt.Errorf("media watch removal is not configured")
	}
	return repository.DeleteMediaPlays(ctx, userID, mediaID)
}

func (service *Service) History(ctx context.Context, userID string, limit int) ([]HistoryEntry, error) {
	if userID == "" {
		return nil, fmt.Errorf("user is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		return nil, fmt.Errorf("limit must not exceed 500")
	}
	repository, ok := service.repository.(historyRepository)
	if !ok {
		return nil, fmt.Errorf("history storage is not configured")
	}
	return repository.ListPlays(ctx, userID, limit)
}

func (service *Service) EpisodeRating(ctx context.Context, userID, episodeID string) (EpisodeRating, error) {
	if userID == "" || episodeID == "" {
		return EpisodeRating{}, fmt.Errorf("user and episode are required")
	}
	repository, ok := service.repository.(episodeRatingRepository)
	if !ok {
		return EpisodeRating{}, fmt.Errorf("episode rating storage is not configured")
	}
	return repository.GetEpisodeRating(ctx, userID, episodeID)
}

func (service *Service) RateEpisode(ctx context.Context, userID, episodeID string, rating *int) (EpisodeRating, error) {
	if userID == "" || episodeID == "" {
		return EpisodeRating{}, fmt.Errorf("user and episode are required")
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return EpisodeRating{}, fmt.Errorf("rating must be from 1 to 5")
	}
	repository, ok := service.repository.(episodeRatingRepository)
	if !ok {
		return EpisodeRating{}, fmt.Errorf("episode rating storage is not configured")
	}
	return repository.SaveEpisodeRating(ctx, userID, episodeID, rating)
}
