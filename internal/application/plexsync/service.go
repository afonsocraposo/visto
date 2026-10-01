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
	ErrWebhookNotFound  = errors.New("Plex webhook is not configured")
	ErrPersonalDisabled = errors.New("personal Plex webhooks are disabled")
	ErrEventNotFound    = errors.New("Plex event not found")
	ErrAccountUnmapped  = errors.New("Plex account is not mapped")
	tmdbGUIDPattern     = regexp.MustCompile(`(?i)(?:tmdb://(?:movie/|tv/)?|agents\.themoviedb://|themoviedb\.org/(?:movie|tv)/)([0-9]+)`)
)

type Event struct {
	ID         int64  `json:"id"`
	TMDBID     int64  `json:"tmdb_id,omitempty"`
	EventType  string `json:"event_type,omitempty"`
	RawPayload string `json:"-"`
	Status     string `json:"status"`
	Title      string `json:"title,omitempty"`
	// EpisodeLabel is "S2E4" for episodes, so the history can name the show and the episode.
	EpisodeLabel string    `json:"episode_label,omitempty"`
	MediaType    string    `json:"media_type,omitempty"`
	Message      string    `json:"message,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type Status struct {
	Mode                  string    `json:"mode"`
	ManagedAccountID      string    `json:"managed_account_id,omitempty"`
	ManagedWebhookEnabled bool      `json:"managed_webhook_enabled"`
	Enabled               bool      `json:"enabled"`
	AccountID             string    `json:"account_id,omitempty"`
	CreatedAt             time.Time `json:"created_at,omitempty"`
	LastUsedAt            time.Time `json:"last_used_at,omitempty"`
	LastSyncedAt          time.Time `json:"last_synced_at,omitempty"`
	RecentEvents          []Event   `json:"recent_events"`
}

type ObservedAccount struct {
	AccountID  string    `json:"account_id"`
	Title      string    `json:"title"`
	LastSeenAt time.Time `json:"last_seen_at"`
	UserID     string    `json:"user_id,omitempty"`
}

type Mapping struct {
	UserID    string `json:"user_id"`
	AccountID string `json:"account_id"`
}

type AdminStatus struct {
	Mode             string            `json:"mode"`
	WebhookEnabled   bool              `json:"webhook_enabled"`
	CreatedAt        time.Time         `json:"created_at,omitempty"`
	LastUsedAt       time.Time         `json:"last_used_at,omitempty"`
	Mappings         []Mapping         `json:"mappings"`
	ObservedAccounts []ObservedAccount `json:"observed_accounts"`
}

type Repository interface {
	IssueSharedPlexWebhook(context.Context, string, time.Time) error
	RevokeSharedPlexWebhook(context.Context) error
	SharedPlexWebhookStatus(context.Context) (bool, time.Time, time.Time, error)
	SharedPlexWebhookByToken(context.Context, string, time.Time) (bool, error)
	PlexUserForAccount(context.Context, string) (string, error)
	ObservePlexAccount(context.Context, string, string, time.Time) error
	ListPlexAccounts(context.Context) ([]ObservedAccount, error)
	ListPlexMappings(context.Context) ([]Mapping, error)
	SetPlexMapping(context.Context, string, string) error
	DeletePlexMapping(context.Context, string) error
	PlexMappingForUser(context.Context, string) (string, error)
	IssuePlexWebhook(context.Context, string, string, string, time.Time) error
	RevokePlexWebhook(context.Context, string) error
	GetPlexWebhookStatus(context.Context, string) (Status, error)
	UserForPlexWebhook(context.Context, string, time.Time) (string, string, error)
	GetPlexEventPayload(context.Context, string, int64) (string, error)
	LogPlexEvent(context.Context, string, string, Event) error
	RecordPlexPlay(context.Context, string, string, Event, *string, *string, time.Time, time.Duration) (bool, error)
}

type MetadataProvider interface {
	Search(context.Context, string, string) ([]domain.MediaSearchResult, error)
	Movie(context.Context, int64) (domain.MovieMetadata, error)
	ShowSummary(context.Context, int64) (domain.TVShowMetadata, error)
	Season(context.Context, int64, int) (domain.TVSeasonMetadata, error)
}

// EpisodeFinder is an optional capability of the metadata provider: it resolves an episode from
// an external ID ("tvdb_id", "imdb_id"). Plex sends these for the episode, so they identify
// the show even when its Plex title does not match TMDB.
type EpisodeFinder interface {
	FindEpisodeByExternalID(ctx context.Context, source, externalID string) (domain.EpisodeLocation, error)
}

// LocalEpisode is an episode already in Visto's catalog.
type LocalEpisode struct {
	EpisodeID  string
	ShowTMDBID int64
	ShowTitle  string
	Season     int
	Episode    int
}

// MatchStore is an optional capability of the repository that lets Plex events be matched
// without asking TMDB, from episodes already in the local catalog.
type MatchStore interface {
	// LocalEpisode finds an episode by show, season and episode number.
	LocalEpisode(ctx context.Context, tmdbShowID int64, season, episode int) (LocalEpisode, bool, error)
	// LocalEpisodeByTMDBID finds an episode by its own TMDB ID, which also names its show.
	LocalEpisodeByTMDBID(ctx context.Context, tmdbEpisodeID int64) (LocalEpisode, bool, error)
}

type Library interface {
	StoreMedia(context.Context, library.Media) error
	ImportShowSeason(context.Context, string, domain.TVShowMetadata, domain.TVSeasonMetadata) error
}

type Service struct {
	repository Repository
	metadata   MetadataProvider
	library    Library
	publicURL  string
	mode       string
	now        func() time.Time
}

func NewService(repository Repository, metadata MetadataProvider, libraryService Library, publicURL string, modes ...string) *Service {
	mode := "personal"
	if len(modes) > 0 {
		mode = modes[0]
	}
	return &Service{repository: repository, metadata: metadata, library: libraryService, publicURL: strings.TrimRight(publicURL, "/"), mode: mode, now: time.Now}
}

func (service *Service) Status(ctx context.Context, userID string) (Status, error) {
	status, err := service.repository.GetPlexWebhookStatus(ctx, userID)
	if err != nil {
		return Status{}, err
	}
	if status.RecentEvents == nil {
		status.RecentEvents = []Event{}
	}
	status.Mode = service.mode
	status.ManagedAccountID, err = service.repository.PlexMappingForUser(ctx, userID)
	if err != nil {
		return Status{}, err
	}
	status.ManagedWebhookEnabled, _, _, err = service.repository.SharedPlexWebhookStatus(ctx)
	if err != nil {
		return Status{}, err
	}
	return status, nil
}

// EventPayload returns the stored raw webhook body of one of the user's events.
func (service *Service) EventPayload(ctx context.Context, userID string, id int64) (string, error) {
	return service.repository.GetPlexEventPayload(ctx, userID, id)
}

func (service *Service) AdminStatus(ctx context.Context) (AdminStatus, error) {
	var status AdminStatus
	var err error
	status.Mode = service.mode
	status.WebhookEnabled, status.CreatedAt, status.LastUsedAt, err = service.repository.SharedPlexWebhookStatus(ctx)
	if err != nil {
		return status, err
	}
	status.Mappings, err = service.repository.ListPlexMappings(ctx)
	if err != nil {
		return status, err
	}
	status.ObservedAccounts, err = service.repository.ListPlexAccounts(ctx)
	return status, err
}

func (service *Service) SetMapping(ctx context.Context, userID, accountID string) error {
	if userID == "" || !validPlexAccountID(accountID) {
		return errors.New("invalid Plex account mapping")
	}
	return service.repository.SetPlexMapping(ctx, userID, accountID)
}

func (service *Service) DeleteMapping(ctx context.Context, userID string) error {
	return service.repository.DeletePlexMapping(ctx, userID)
}

func (service *Service) Issue(ctx context.Context, userID, accountID string) (string, error) {
	if service.mode != "personal" {
		return "", ErrPersonalDisabled
	}
	accountID = strings.TrimSpace(accountID)
	if accountID != "" && !validPlexAccountID(accountID) {
		return "", errors.New("Plex account ID must contain at most 20 digits")
	}
	if err := service.validatePublicURL(); err != nil {
		return "", err
	}
	secret, hash, err := newWebhookSecret()
	if err != nil {
		return "", err
	}
	if err := service.repository.IssuePlexWebhook(ctx, userID, hash, accountID, service.now().UTC()); err != nil {
		return "", err
	}
	return service.publicURL + "/api/v1/webhooks/plex/" + secret, nil
}

func (service *Service) IssueShared(ctx context.Context) (string, error) {
	if err := service.validatePublicURL(); err != nil {
		return "", err
	}
	secret, hash, err := newWebhookSecret()
	if err != nil {
		return "", err
	}
	if err := service.repository.IssueSharedPlexWebhook(ctx, hash, service.now().UTC()); err != nil {
		return "", err
	}
	return service.publicURL + "/api/v1/webhooks/plex/" + secret, nil
}

func (service *Service) RevokeShared(ctx context.Context) error {
	return service.repository.RevokeSharedPlexWebhook(ctx)
}

func (service *Service) validatePublicURL() error {
	if service.publicURL == "" {
		return errors.New("VISTO_PUBLIC_URL must be configured to create a Plex webhook URL")
	}
	u, err := url.Parse(service.publicURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("VISTO_PUBLIC_URL must be a public HTTPS origin without a path")
	}
	return nil
}

func newWebhookSecret() (string, string, error) {
	value := make([]byte, secretBytes)
	if _, err := rand.Read(value); err != nil {
		return "", "", fmt.Errorf("generate Plex webhook secret: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(value)
	hash := sha256.Sum256([]byte(secret))
	return secret, hex.EncodeToString(hash[:]), nil
}

func (service *Service) Revoke(ctx context.Context, userID string) error {
	if service.mode != "personal" {
		return ErrPersonalDisabled
	}
	return service.repository.RevokePlexWebhook(ctx, userID)
}

func (service *Service) Handle(ctx context.Context, secret, rawPayload string) error {
	if len(rawPayload) == 0 || len(rawPayload) > maxPayloadBytes {
		return errors.New("invalid Plex webhook payload size")
	}
	hash := sha256.Sum256([]byte(secret))
	mode := service.mode
	var err error
	var userID, accountID string
	if mode == "managed" {
		valid, lookupErr := service.repository.SharedPlexWebhookByToken(ctx, hex.EncodeToString(hash[:]), service.now().UTC())
		if lookupErr != nil {
			return lookupErr
		}
		if !valid {
			return ErrWebhookNotFound
		}
	} else {
		userID, accountID, err = service.repository.UserForPlexWebhook(ctx, hex.EncodeToString(hash[:]), service.now().UTC())
		if err != nil {
			return ErrWebhookNotFound
		}
	}
	var payload plexPayload
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		if mode == "managed" {
			return nil
		}
		return service.log(ctx, userID, service.rawFingerprint(rawPayload), Event{RawPayload: rawPayload, Status: "failed", Message: "Plex sent invalid event data", OccurredAt: service.now().UTC()})
	}
	if mode == "managed" {
		accountID = payload.Account.ID.String()
		if !validPlexAccountID(accountID) {
			return nil
		}
		if strings.HasPrefix(payload.Event, "media.") {
			if err := service.repository.ObservePlexAccount(ctx, accountID, payload.Account.Title, service.now().UTC()); err != nil {
				return err
			}
		}
		userID, err = service.repository.PlexUserForAccount(ctx, accountID)
		if errors.Is(err, ErrAccountUnmapped) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	metadata := payload.Metadata
	event := Event{EventType: payload.Event, RawPayload: rawPayload, Status: "skipped", Title: metadata.displayTitle(), EpisodeLabel: metadata.episodeLabel(), MediaType: metadata.Type, OccurredAt: plexTime(metadata.LastViewedAt, service.now().UTC())}
	fingerprint := service.fingerprint(payload, rawPayload)
	if accountID == "" || payload.Account.ID == "" || payload.Account.ID.String() != accountID {
		event.Message = "Plex account does not match this webhook"
		if validPlexAccountID(payload.Account.ID.String()) {
			event.Message = "Skipped Plex account ID " + payload.Account.ID.String()
		}
		return service.log(ctx, userID, fingerprint, event)
	}
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
		if err := service.library.StoreMedia(ctx, media); err != nil {
			event.Status, event.Message = "failed", "Could not add the movie to the Visto library"
			return service.log(ctx, userID, fingerprint, event)
		}
		id := media.ID
		mediaID, event.Title, event.MediaType, event.TMDBID = &id, movie.Title, "movie", movie.TMDBID
	case domain.TVMediaType, "episode":
		resolved, resolveErr := service.resolveEpisode(ctx, metadata)
		if resolveErr != nil {
			return service.logResolutionError(ctx, userID, fingerprint, event, resolveErr)
		}
		if resolved.fetched != nil {
			// Only fetched metadata needs storing; a locally known episode is already catalogued.
			show, season := resolved.fetched.show, resolved.fetched.season
			media := library.Media{ID: fmt.Sprintf("tv:%d", show.TMDBID), Type: domain.TVMediaType, TMDBID: show.TMDBID, Title: show.Name, OriginalTitle: show.Name, Overview: show.Overview, ReleaseDate: show.FirstAirDate, PosterPath: show.PosterPath, BackdropPath: show.BackdropPath, OriginalLanguage: show.OriginalLanguage, Status: show.Status}
			if err := service.library.StoreMedia(ctx, media); err != nil {
				event.Status, event.Message = "failed", "Could not add the TV show to the Visto library"
				return service.log(ctx, userID, fingerprint, event)
			}
			if err := service.library.ImportShowSeason(ctx, userID, show, season); err != nil {
				event.Status, event.Message = "failed", "Could not save the TV episode metadata"
				return service.log(ctx, userID, fingerprint, event)
			}
		}
		id := resolved.episodeID
		episodeID, event.Title, event.MediaType, event.TMDBID = &id, resolved.showTitle, "tv", resolved.showTMDBID
		event.EpisodeLabel = episodeLabel(resolved.season, resolved.episode)
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
	Account  plexAccount  `json:"Account"`
	Server   plexIdentity `json:"Server"`
	Player   plexIdentity `json:"Player"`
	Metadata plexMetadata `json:"Metadata"`
}

type plexAccount struct {
	ID    json.Number `json:"id"`
	Title string      `json:"title"`
}

func validPlexAccountID(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
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

// displayTitle names what the history shows: the show for an episode (never the episode's own
// title), the movie title for a movie.
func (metadata plexMetadata) displayTitle() string {
	if metadata.Type == "episode" {
		show, _ := splitPlexTitle(metadata.GrandparentTitle)
		return firstNonEmpty(show, metadata.Title)
	}
	return firstNonEmpty(metadata.Title, metadata.GrandparentTitle)
}

func (metadata plexMetadata) episodeLabel() string {
	if metadata.Type != "episode" || metadata.ParentIndex == nil || metadata.EpisodeIndex == nil || *metadata.ParentIndex < 0 || *metadata.EpisodeIndex <= 0 {
		return ""
	}
	return episodeLabel(*metadata.ParentIndex, *metadata.EpisodeIndex)
}

func episodeLabel(season, episode int) string {
	return fmt.Sprintf("S%dE%d", season, episode)
}

// plexYearSuffix matches the "(2025)" Plex appends to a title to tell similarly named titles
// apart. TMDB titles do not carry it.
var plexYearSuffix = regexp.MustCompile(`\s*\((\d{4})\)\s*$`)

// splitPlexTitle removes a trailing "(YYYY)" and returns it as a year hint (0 when absent).
func splitPlexTitle(title string) (string, int) {
	title = strings.TrimSpace(title)
	match := plexYearSuffix.FindStringSubmatchIndex(title)
	if match == nil {
		return title, 0
	}
	year, _ := strconv.Atoi(title[match[2]:match[3]])
	return strings.TrimSpace(title[:match[0]]), year
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

type fetchedEpisode struct {
	show   domain.TVShowMetadata
	season domain.TVSeasonMetadata
}

// resolvedEpisode is a Plex episode matched to Visto's catalog. fetched is set only when TMDB had
// to be asked, in which case that metadata still has to be stored.
type resolvedEpisode struct {
	showTMDBID int64
	showTitle  string
	season     int
	episode    int
	episodeID  string
	fetched    *fetchedEpisode
}

func (found LocalEpisode) resolved() resolvedEpisode {
	return resolvedEpisode{showTMDBID: found.ShowTMDBID, showTitle: found.ShowTitle, season: found.Season, episode: found.Episode, episodeID: found.EpisodeID}
}

// resolveEpisode matches a Plex episode, asking TMDB as little as possible, in this order:
//  1. the episode's own TMDB ID (Plex sends it) in the local catalog, which names the show (no request);
//  2. a TMDB ID on the show's GUID (older Plex agents), then the episode by number in the catalog (no request);
//  3. the episode's TVDB/IMDb ID through TMDB's find endpoint (one request);
//  4. an exact title search, using the year Plex puts in the title (one request).
//
// Whatever TMDB had to supply is stored, so the show's later episodes are found locally.
func (service *Service) resolveEpisode(ctx context.Context, metadata plexMetadata) (resolvedEpisode, error) {
	var season, episode int
	havePosition := metadata.ParentIndex != nil && metadata.EpisodeIndex != nil && *metadata.ParentIndex >= 0 && *metadata.EpisodeIndex > 0
	if havePosition {
		season, episode = *metadata.ParentIndex, *metadata.EpisodeIndex
	}
	local, _ := service.repository.(MatchStore)
	if local != nil {
		for _, episodeID := range tmdbIDs(metadata.GUIDs) {
			if found, ok, err := local.LocalEpisodeByTMDBID(ctx, episodeID); err == nil && ok {
				return found.resolved(), nil
			}
		}
	}
	byPosition := func(showID int64, season, episode int) (resolvedEpisode, bool) {
		if local == nil || showID == 0 {
			return resolvedEpisode{}, false
		}
		found, ok, err := local.LocalEpisode(ctx, showID, season, episode)
		if err != nil || !ok {
			return resolvedEpisode{}, false
		}
		return found.resolved(), true
	}
	showID := parseTMDBGUID(metadata.GrandparentGUID)
	if havePosition {
		if resolved, ok := byPosition(showID, season, episode); ok {
			return resolved, nil
		}
	}
	if showID == 0 {
		if location, found := service.findEpisode(ctx, metadata); found {
			showID, season, episode, havePosition = location.ShowTMDBID, location.SeasonNumber, location.EpisodeNumber, true
			if resolved, ok := byPosition(showID, season, episode); ok {
				return resolved, nil
			}
		}
	}
	if showID == 0 {
		year := metadata.GrandparentYear
		if year == 0 {
			year = metadata.ParentYear
		}
		var err error
		showID, err = service.searchID(ctx, domain.TVMediaType, metadata.GrandparentTitle, "", year)
		if err != nil {
			return resolvedEpisode{}, err
		}
	}
	if !havePosition {
		return resolvedEpisode{}, fmt.Errorf("Plex did not include a valid season and episode number")
	}
	show, err := service.metadata.ShowSummary(ctx, showID)
	if err != nil {
		return resolvedEpisode{}, fmt.Errorf("TMDB could not load the TV show")
	}
	seasonData, err := service.metadata.Season(ctx, showID, season)
	if err != nil {
		return resolvedEpisode{}, fmt.Errorf("TMDB could not load the matching season")
	}
	for _, candidate := range seasonData.Episodes {
		if candidate.SeasonNumber == season && candidate.EpisodeNumber == episode {
			return resolvedEpisode{
				showTMDBID: show.TMDBID, showTitle: show.Name, season: season, episode: episode,
				episodeID: fmt.Sprintf("tv:%d:episode:%d", show.TMDBID, candidate.TMDBID),
				fetched:   &fetchedEpisode{show: show, season: seasonData},
			}, nil
		}
	}
	return resolvedEpisode{}, fmt.Errorf("no exact TMDB episode matched Plex season %d episode %d", season, episode)
}

// tmdbIDs returns every TMDB ID in Plex's Guid list. For an episode these are the episode's own IDs.
func tmdbIDs(raw json.RawMessage) []int64 {
	var list []struct {
		ID string `json:"id"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var ids []int64
	for _, entry := range list {
		if id := parseTMDBGUID(strings.TrimSpace(entry.ID)); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// findEpisode looks the episode up by the external IDs Plex lists for it. A lookup that fails or
// is ambiguous simply falls through to the next strategy.
func (service *Service) findEpisode(ctx context.Context, metadata plexMetadata) (domain.EpisodeLocation, bool) {
	finder, ok := service.metadata.(EpisodeFinder)
	if !ok {
		return domain.EpisodeLocation{}, false
	}
	for _, candidate := range externalEpisodeIDs(metadata.GUIDs) {
		location, err := finder.FindEpisodeByExternalID(ctx, candidate.source, candidate.id)
		if err == nil && location.ShowTMDBID > 0 && location.EpisodeNumber > 0 {
			return location, true
		}
	}
	return domain.EpisodeLocation{}, false
}

type externalID struct{ source, id string }

var externalGUIDPattern = regexp.MustCompile(`(?i)^(tvdb|imdb)://(tt)?([0-9]+)$`)

// externalEpisodeIDs returns the TVDB and IMDb IDs from Plex's Guid list, TVDB first.
func externalEpisodeIDs(raw json.RawMessage) []externalID {
	var list []struct {
		ID string `json:"id"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var tvdb, imdb []externalID
	for _, entry := range list {
		match := externalGUIDPattern.FindStringSubmatch(strings.TrimSpace(entry.ID))
		if match == nil {
			continue
		}
		if strings.EqualFold(match[1], "tvdb") {
			tvdb = append(tvdb, externalID{"tvdb_id", match[3]})
		} else {
			imdb = append(imdb, externalID{"imdb_id", "tt" + match[3]})
		}
	}
	return append(tvdb, imdb...)
}

func (service *Service) searchID(ctx context.Context, mediaType domain.MediaType, title, originalTitle string, year int) (int64, error) {
	// Plex may title things "Show (2025)"; search for the bare title and use that year.
	title, titleYear := splitPlexTitle(title)
	if year == 0 {
		year = titleYear
	}
	titles := []string{title}
	if originalTitle != "" {
		if original, _ := splitPlexTitle(originalTitle); normalizedTitle(original) != normalizedTitle(title) {
			titles = append(titles, original)
		}
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

// parseTMDBGUID reads the ID from a plain GUID string such as "tmdb://1396" or a TMDB URL.
func parseTMDBGUID(value string) int64 {
	if matches := tmdbGUIDPattern.FindStringSubmatch(value); len(matches) == 2 {
		id, _ := strconv.ParseInt(matches[1], 10, 64)
		return id
	}
	return 0
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
