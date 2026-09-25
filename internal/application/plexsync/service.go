package plexsync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/domain"
)

const (
	maxPayloadBytes = 1 << 20
	duplicateWindow = 7 * 24 * time.Hour
	secretBytes     = 32
)

var (
	ErrWebhookNotFound = errors.New("Plex webhook is not configured")
	tmdbGUIDPattern    = regexp.MustCompile(`(?i)(?:tmdb://(?:movie/|tv/)?|themoviedb\.org/(?:movie|tv)/)([0-9]+)`)
)

type Event struct {
	ID         int64     `json:"id"`
	TMDBID     int64     `json:"tmdb_id,omitempty"`
	Status     string    `json:"status"`
	Title      string    `json:"title,omitempty"`
	MediaType  string    `json:"media_type,omitempty"`
	Message    string    `json:"message,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type Status struct {
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	LastUsedAt   time.Time `json:"last_used_at,omitempty"`
	LastSyncedAt time.Time `json:"last_synced_at,omitempty"`
	RecentEvents []Event   `json:"recent_events"`
}

type Repository interface {
	IssuePlexWebhook(context.Context, string, string, time.Time) error
	RevokePlexWebhook(context.Context, string) error
	GetPlexWebhookStatus(context.Context, string) (Status, error)
	UserForPlexWebhook(context.Context, string, time.Time) (string, error)
	LogPlexEvent(context.Context, string, string, Event) error
	RecordPlexPlay(context.Context, string, string, Event, *string, *string, time.Time, time.Duration) (bool, error)
}

type MetadataProvider interface {
	Search(context.Context, string, string) ([]domain.MediaSearchResult, error)
	Movie(context.Context, int64) (domain.MovieMetadata, error)
	ShowSummary(context.Context, int64) (domain.TVShowMetadata, error)
	Season(context.Context, int64, int) (domain.TVSeasonMetadata, error)
}

type Library interface {
	SaveMedia(context.Context, string, library.Media, domain.LibraryStatus, *int) (library.Item, error)
	ImportShowSeason(context.Context, string, domain.TVShowMetadata, domain.TVSeasonMetadata) error
}

type Service struct {
	repository Repository
	metadata   MetadataProvider
	library    Library
	publicURL  string
	now        func() time.Time
}

func NewService(repository Repository, metadata MetadataProvider, libraryService Library, publicURL string) *Service {
	return &Service{repository: repository, metadata: metadata, library: libraryService, publicURL: strings.TrimRight(publicURL, "/"), now: time.Now}
}

func (service *Service) Status(ctx context.Context, userID string) (Status, error) {
	status, err := service.repository.GetPlexWebhookStatus(ctx, userID)
	if err != nil {
		return Status{}, err
	}
	if status.RecentEvents == nil {
		status.RecentEvents = []Event{}
	}
	return status, nil
}

func (service *Service) Issue(ctx context.Context, userID string) (string, error) {
	if service.publicURL == "" {
		return "", errors.New("VISTO_PUBLIC_URL must be configured to create a Plex webhook URL")
	}
	publicURL, err := url.Parse(service.publicURL)
	if err != nil || publicURL.Scheme != "https" || publicURL.Host == "" || publicURL.User != nil || (publicURL.Path != "" && publicURL.Path != "/") || publicURL.RawQuery != "" || publicURL.Fragment != "" {
		return "", errors.New("VISTO_PUBLIC_URL must be a public HTTPS origin without a path")
	}
	secretBytesValue := make([]byte, secretBytes)
	if _, err := rand.Read(secretBytesValue); err != nil {
		return "", fmt.Errorf("generate Plex webhook secret: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytesValue)
	hash := sha256.Sum256([]byte(secret))
	if err := service.repository.IssuePlexWebhook(ctx, userID, hex.EncodeToString(hash[:]), service.now().UTC()); err != nil {
		return "", err
	}
	return service.publicURL + "/api/v1/webhooks/plex/" + secret, nil
}

func (service *Service) Revoke(ctx context.Context, userID string) error {
	return service.repository.RevokePlexWebhook(ctx, userID)
}

func (service *Service) Handle(ctx context.Context, secret, rawPayload string) error {
	if len(rawPayload) == 0 || len(rawPayload) > maxPayloadBytes {
		return errors.New("invalid Plex webhook payload size")
	}
	hash := sha256.Sum256([]byte(secret))
	userID, err := service.repository.UserForPlexWebhook(ctx, hex.EncodeToString(hash[:]), service.now().UTC())
	if err != nil {
		return ErrWebhookNotFound
	}
	var payload plexPayload
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		return service.log(ctx, userID, service.rawFingerprint(rawPayload), Event{Status: "failed", Message: "Plex sent invalid event data", OccurredAt: service.now().UTC()})
	}
	metadata := payload.Metadata
	event := Event{Status: "skipped", Title: firstNonEmpty(metadata.Title, metadata.GrandparentTitle), MediaType: metadata.Type, OccurredAt: plexTime(metadata.LastViewedAt, service.now().UTC())}
	fingerprint := service.fingerprint(payload, rawPayload)
	if payload.Event != "media.scrobble" {
		event.Message = "Only Plex watched events are synchronized"
		return service.log(ctx, userID, fingerprint, event)
	}
	if service.metadata == nil || service.library == nil {
		event.Status, event.Message = "failed", "Media metadata is not configured on this Visto instance"
		return service.log(ctx, userID, fingerprint, event)
	}
	mediaType := domain.MediaType(metadata.Type)
	var mediaID, episodeID *string
	switch mediaType {
	case domain.MovieMediaType:
		movie, resolveErr := service.resolveMovie(ctx, metadata)
		if resolveErr != nil {
			return service.logResolutionError(ctx, userID, fingerprint, event, resolveErr)
		}
		media := library.Media{ID: fmt.Sprintf("movie:%d", movie.TMDBID), Type: domain.MovieMediaType, TMDBID: movie.TMDBID, Title: movie.Title, OriginalTitle: movie.OriginalTitle, Overview: movie.Overview, ReleaseDate: movie.ReleaseDate, PosterPath: movie.PosterPath, BackdropPath: movie.BackdropPath, OriginalLanguage: movie.OriginalLanguage, Status: movie.Status}
		if _, err := service.library.SaveMedia(ctx, userID, media, domain.WatchingStatus, nil); err != nil {
			event.Status, event.Message = "failed", "Could not add the movie to the Visto library"
			return service.log(ctx, userID, fingerprint, event)
		}
		id := media.ID
		mediaID, event.Title, event.MediaType, event.TMDBID = &id, movie.Title, "movie", movie.TMDBID
	case domain.TVMediaType, "episode":
		show, season, matchedEpisode, resolveErr := service.resolveEpisode(ctx, metadata)
		if resolveErr != nil {
			return service.logResolutionError(ctx, userID, fingerprint, event, resolveErr)
		}
		media := library.Media{ID: fmt.Sprintf("tv:%d", show.TMDBID), Type: domain.TVMediaType, TMDBID: show.TMDBID, Title: show.Name, OriginalTitle: show.Name, Overview: show.Overview, ReleaseDate: show.FirstAirDate, PosterPath: show.PosterPath, BackdropPath: show.BackdropPath, OriginalLanguage: show.OriginalLanguage, Status: show.Status}
		if _, err := service.library.SaveMedia(ctx, userID, media, domain.WatchingStatus, nil); err != nil {
			event.Status, event.Message = "failed", "Could not add the TV show to the Visto library"
			return service.log(ctx, userID, fingerprint, event)
		}
		if err := service.library.ImportShowSeason(ctx, userID, show, season); err != nil {
			event.Status, event.Message = "failed", "Could not save the TV episode metadata"
			return service.log(ctx, userID, fingerprint, event)
		}
		id := fmt.Sprintf("tv:%d:episode:%d", show.TMDBID, matchedEpisode.TMDBID)
		episodeID, event.Title, event.MediaType, event.TMDBID = &id, show.Name, "tv", show.TMDBID
	default:
		event.Message = "Plex event is not a movie or TV episode"
		return service.log(ctx, userID, fingerprint, event)
	}
	event.Status = "synced"
	event.Message = "Watched content synchronized"
	recorded, err := service.repository.RecordPlexPlay(ctx, userID, fingerprint, event, mediaID, episodeID, event.OccurredAt, duplicateWindow)
	if err != nil {
		return err
	}
	if !recorded {
		return nil
	}
	return nil
}

type plexPayload struct {
	Event    string       `json:"event"`
	Account  any          `json:"Account"`
	Server   plexIdentity `json:"Server"`
	Player   plexIdentity `json:"Player"`
	Metadata plexMetadata `json:"Metadata"`
}

type plexIdentity struct {
	UUID string `json:"uuid"`
}

type plexMetadata struct {
	Type             string          `json:"type"`
	Title            string          `json:"title"`
	OriginalTitle    string          `json:"originalTitle"`
	Year             int             `json:"year"`
	GUID             json.RawMessage `json:"guid"`
	GUIDs            json.RawMessage `json:"Guid"`
	GrandparentGUID  string          `json:"grandparentGuid"`
	GrandparentTitle string          `json:"grandparentTitle"`
	ParentIndex      *int            `json:"parentIndex"`
	ParentYear       int             `json:"parentYear"`
	GrandparentYear  int             `json:"grandparentYear"`
	EpisodeIndex     *int            `json:"index"`
	LastViewedAt     json.RawMessage `json:"lastViewedAt"`
}

func (service *Service) resolveMovie(ctx context.Context, metadata plexMetadata) (domain.MovieMetadata, error) {
	id := parseTMDBID(metadata.GUID)
	if id == 0 {
		id = parseTMDBID(metadata.GUIDs)
	}
	if id == 0 {
		var err error
		id, err = service.searchID(ctx, domain.MovieMediaType, metadata.Title, metadata.OriginalTitle, metadata.Year)
		if err != nil {
			return domain.MovieMetadata{}, err
		}
	}
	movie, err := service.metadata.Movie(ctx, id)
	if err != nil {
		return domain.MovieMetadata{}, fmt.Errorf("TMDB could not load the movie")
	}
	return movie, nil
}

func (service *Service) resolveEpisode(ctx context.Context, metadata plexMetadata) (domain.TVShowMetadata, domain.TVSeasonMetadata, domain.TVEpisodeMetadata, error) {
	showID := parseTMDBID(json.RawMessage(metadata.GrandparentGUID))
	if showID == 0 {
		var err error
		year := metadata.GrandparentYear
		if year == 0 {
			year = metadata.ParentYear
		}
		showID, err = service.searchID(ctx, domain.TVMediaType, metadata.GrandparentTitle, "", year)
		if err != nil {
			return domain.TVShowMetadata{}, domain.TVSeasonMetadata{}, domain.TVEpisodeMetadata{}, err
		}
	}
	show, err := service.metadata.ShowSummary(ctx, showID)
	if err != nil {
		return domain.TVShowMetadata{}, domain.TVSeasonMetadata{}, domain.TVEpisodeMetadata{}, fmt.Errorf("TMDB could not load the TV show")
	}
	if metadata.ParentIndex == nil || metadata.EpisodeIndex == nil || *metadata.ParentIndex < 0 || *metadata.EpisodeIndex <= 0 {
		return domain.TVShowMetadata{}, domain.TVSeasonMetadata{}, domain.TVEpisodeMetadata{}, fmt.Errorf("Plex did not include a valid season and episode number")
	}
	seasonNumber, episodeNumber := *metadata.ParentIndex, *metadata.EpisodeIndex
	season, err := service.metadata.Season(ctx, showID, seasonNumber)
	if err != nil {
		return domain.TVShowMetadata{}, domain.TVSeasonMetadata{}, domain.TVEpisodeMetadata{}, fmt.Errorf("TMDB could not load the matching season")
	}
	for _, episode := range season.Episodes {
		if episode.SeasonNumber == seasonNumber && episode.EpisodeNumber == episodeNumber {
			return show, season, episode, nil
		}
	}
	return domain.TVShowMetadata{}, domain.TVSeasonMetadata{}, domain.TVEpisodeMetadata{}, fmt.Errorf("no exact TMDB episode matched Plex season %d episode %d", seasonNumber, episodeNumber)
}

func (service *Service) searchID(ctx context.Context, mediaType domain.MediaType, title, originalTitle string, year int) (int64, error) {
	titles := []string{title}
	if originalTitle != "" && normalizedTitle(originalTitle) != normalizedTitle(title) {
		titles = append(titles, originalTitle)
	}
	for _, query := range titles {
		if strings.TrimSpace(query) == "" {
			continue
		}
		results, err := service.metadata.Search(ctx, query, "en-US")
		if err != nil {
			return 0, fmt.Errorf("TMDB search is temporarily unavailable")
		}
		matches := make(map[int64]bool)
		for _, result := range results {
			if result.Type != mediaType || result.TMDBID <= 0 || normalizedTitle(result.Title) != normalizedTitle(query) && normalizedTitle(result.OriginalTitle) != normalizedTitle(query) {
				continue
			}
			if year > 0 && (len(result.ReleaseDate) < 4 || result.ReleaseDate[:4] != strconv.Itoa(year)) {
				continue
			}
			matches[result.TMDBID] = true
		}
		if len(matches) == 1 {
			for id := range matches {
				return id, nil
			}
		}
		if len(matches) > 1 {
			return 0, fmt.Errorf("TMDB found more than one exact %s match", mediaType)
		}
	}
	return 0, fmt.Errorf("no exact TMDB match for Plex title")
}

func normalizedTitle(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func parseTMDBID(raw json.RawMessage) int64 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		if matches := tmdbGUIDPattern.FindStringSubmatch(value); len(matches) == 2 {
			id, _ := strconv.ParseInt(matches[1], 10, 64)
			return id
		}
		if parsed, err := url.Parse(value); err == nil {
			if matches := tmdbGUIDPattern.FindStringSubmatch(parsed.String()); len(matches) == 2 {
				id, _ := strconv.ParseInt(matches[1], 10, 64)
				return id
			}
		}
		return 0
	}
	var list []struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &list) == nil {
		for _, entry := range list {
			if matches := tmdbGUIDPattern.FindStringSubmatch(entry.ID); len(matches) == 2 {
				id, _ := strconv.ParseInt(matches[1], 10, 64)
				return id
			}
		}
	}
	return 0
}

func plexTime(raw json.RawMessage, fallback time.Time) time.Time {
	if len(raw) == 0 || string(raw) == "null" {
		return fallback
	}
	var seconds int64
	if json.Unmarshal(raw, &seconds) != nil {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return fallback
		}
		seconds, _ = strconv.ParseInt(text, 10, 64)
	}
	if seconds <= 0 {
		return fallback
	}
	parsed := time.Unix(seconds, 0).UTC()
	if parsed.After(fallback) {
		return fallback
	}
	return parsed
}

func (service *Service) fingerprint(payload plexPayload, raw string) string {
	identity := strings.Join([]string{payload.Server.UUID, payload.Player.UUID, payload.Metadata.Type, raw}, "\x00")
	// Plex has no stable delivery ID. A short receipt-time bucket suppresses
	// immediate retries while allowing a later legitimate rewatch event.
	identity += "\x00" + service.now().UTC().Truncate(10*time.Minute).Format(time.RFC3339)
	hash := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(hash[:])
}

func (service *Service) rawFingerprint(raw string) string {
	identity := raw + "\x00" + service.now().UTC().Truncate(10*time.Minute).Format(time.RFC3339)
	hash := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(hash[:])
}

func (service *Service) log(ctx context.Context, userID, fingerprint string, event Event) error {
	return service.repository.LogPlexEvent(ctx, userID, fingerprint, event)
}

func (service *Service) logResolutionError(ctx context.Context, userID, fingerprint string, event Event, err error) error {
	message := err.Error()
	if strings.Contains(message, "temporarily unavailable") || strings.Contains(message, "could not load") {
		event.Status = "failed"
	} else {
		event.Status = "skipped"
	}
	event.Message = message
	return service.log(ctx, userID, fingerprint, event)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
