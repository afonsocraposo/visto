package tmdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	tmdbapi "github.com/cyruzin/golang-tmdb"
	"golang.org/x/sync/singleflight"
)

const (
	defaultRequestTimeout  = 10 * time.Second
	searchCacheTTL         = 5 * time.Minute
	showCacheTTL           = 24 * time.Hour
	maxAttempts            = 3
	maxSearchCacheEntries  = 500
	maxShowCacheEntries    = 100
	relatedCacheTTL        = 12 * time.Hour
	maxRelatedCacheEntries = 300
)

// Client is Visto's TMDB adapter. The third-party client is deliberately kept
// inside this package, so it cannot leak into application or domain code.
type Client struct {
	client       *tmdbapi.Client
	requests     chan struct{}
	mu           sync.Mutex
	calls        singleflight.Group
	pausedUntil  time.Time
	retryAfter   time.Duration
	searchCache  map[string]cachedSearch
	showCache    map[int64]cachedShow
	summaryCache map[int64]cachedShow
	seasonCache  map[string]cachedSeason
	movieCache   map[int64]cachedMovie
	episodeCache map[string]cachedEpisode
	personCache  map[int64]cachedPerson
	relatedCache map[string]cachedSearch
}

type cachedSearch struct {
	results   []domain.MediaSearchResult
	expiresAt time.Time
}

type cachedShow struct {
	show      domain.TVShowMetadata
	expiresAt time.Time
}
type cachedSeason struct {
	season    domain.TVSeasonMetadata
	expiresAt time.Time
}
type cachedMovie struct {
	movie     domain.MovieMetadata
	expiresAt time.Time
}
type cachedEpisode struct {
	episode   domain.TVEpisodeMetadata
	expiresAt time.Time
}
type cachedPerson struct {
	person    domain.PersonMetadata
	expiresAt time.Time
}

type retryAfterTransport struct {
	next    http.RoundTripper
	onLimit func(string)
}

func (transport retryAfterTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.next.RoundTrip(request)
	if err == nil && response.StatusCode == http.StatusTooManyRequests && transport.onLimit != nil {
		transport.onLimit(response.Header.Get("Retry-After"))
	}
	if err == nil && response.StatusCode == http.StatusTooManyRequests {
		_ = response.Body.Close()
		response.Body = io.NopCloser(strings.NewReader(`{"status_code":429,"status_message":"rate limited","success":false}`))
		response.ContentLength = int64(len(`{"status_code":429,"status_message":"rate limited","success":false}`))
	}
	return response, err
}

func New(apiKey string, httpClient *http.Client) (*Client, error) {
	client, err := tmdbapi.Init(apiKey)
	if err != nil {
		return nil, fmt.Errorf("initialize TMDB client: %w", err)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout, Transport: &http.Transport{MaxConnsPerHost: 4, MaxIdleConnsPerHost: 4}}
	}
	if httpClient.Timeout <= 0 {
		httpClient.Timeout = defaultRequestTimeout
	}
	if httpClient.Transport == nil {
		bounded := http.DefaultTransport.(*http.Transport).Clone()
		bounded.MaxConnsPerHost = 4
		bounded.MaxIdleConnsPerHost = 4
		httpClient.Transport = bounded
	}
	vistoClient := &Client{client: client, requests: make(chan struct{}, 4), searchCache: map[string]cachedSearch{}, showCache: map[int64]cachedShow{}, summaryCache: map[int64]cachedShow{}, seasonCache: map[string]cachedSeason{}, movieCache: map[int64]cachedMovie{}, episodeCache: map[string]cachedEpisode{}, personCache: map[int64]cachedPerson{}, relatedCache: map[string]cachedSearch{}}
	baseTransport := httpClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	httpClient.Transport = retryAfterTransport{next: baseTransport, onLimit: vistoClient.captureRetryAfter}
	client.SetClientConfig(*httpClient)
	return vistoClient, nil
}

func (client *Client) Search(ctx context.Context, query, language string) ([]domain.MediaSearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	language = strings.TrimSpace(language)
	if query == "" {
		return []domain.MediaSearchResult{}, nil
	}
	cacheKey := language + "\x00" + query
	client.mu.Lock()
	if cached, ok := client.searchCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return append([]domain.MediaSearchResult(nil), cached.results...), nil
	}
	client.mu.Unlock()

	result, err := client.coalesce(ctx, "search:"+cacheKey, func() (any, error) {
		results, err := client.search(ctx, query, language)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.searchCache[cacheKey] = cachedSearch{results: append([]domain.MediaSearchResult(nil), results...), expiresAt: time.Now().Add(searchCacheTTL)}
		if len(client.searchCache) > maxSearchCacheEntries {
			now := time.Now()
			for key, cached := range client.searchCache {
				if now.After(cached.expiresAt) {
					delete(client.searchCache, key)
				}
			}
			if len(client.searchCache) > maxSearchCacheEntries {
				for key := range client.searchCache {
					delete(client.searchCache, key)
					break
				}
			}
		}
		client.mu.Unlock()
		return results, nil
	})
	if err != nil {
		return nil, err
	}
	return append([]domain.MediaSearchResult(nil), result.([]domain.MediaSearchResult)...), nil
}

func (client *Client) Trending(ctx context.Context, mediaType, timeWindow string) ([]domain.MediaSearchResult, error) {
	if mediaType != "movie" && mediaType != "tv" {
		return nil, fmt.Errorf("trending media type must be movie or tv")
	}
	if timeWindow != "day" && timeWindow != "week" {
		return nil, fmt.Errorf("trending time window must be day or week")
	}
	cacheKey := "trending\x00" + mediaType + "\x00" + timeWindow
	client.mu.Lock()
	if cached, ok := client.searchCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return append([]domain.MediaSearchResult(nil), cached.results...), nil
	}
	client.mu.Unlock()

	result, err := client.coalesce(ctx, "trending:"+cacheKey, func() (any, error) {
		results, err := client.fetchTrending(ctx, mediaType, timeWindow)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.searchCache[cacheKey] = cachedSearch{results: append([]domain.MediaSearchResult(nil), results...), expiresAt: time.Now().Add(searchCacheTTL)}
		client.mu.Unlock()
		return results, nil
	})
	if err != nil {
		return nil, err
	}
	return append([]domain.MediaSearchResult(nil), result.([]domain.MediaSearchResult)...), nil
}

func (client *Client) Related(ctx context.Context, mediaType domain.MediaType, tmdbID int64) ([]domain.MediaSearchResult, error) {
	if (mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType) || tmdbID <= 0 {
		return nil, fmt.Errorf("related media type and TMDB ID must be valid")
	}
	cacheKey := fmt.Sprintf("%s:%d", mediaType, tmdbID)
	client.mu.Lock()
	if cached, ok := client.relatedCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return append([]domain.MediaSearchResult(nil), cached.results...), nil
	}
	client.mu.Unlock()

	result, err := client.coalesce(ctx, "related:"+cacheKey, func() (any, error) {
		results, err := client.fetchRelated(ctx, mediaType, tmdbID)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		now := time.Now()
		for key, cached := range client.relatedCache {
			if !now.Before(cached.expiresAt) {
				delete(client.relatedCache, key)
			}
		}
		client.relatedCache[cacheKey] = cachedSearch{results: append([]domain.MediaSearchResult(nil), results...), expiresAt: now.Add(relatedCacheTTL)}
		for len(client.relatedCache) > maxRelatedCacheEntries {
			for key := range client.relatedCache {
				if key != cacheKey {
					delete(client.relatedCache, key)
					break
				}
			}
		}
		client.mu.Unlock()
		return results, nil
	})
	if err != nil {
		return nil, err
	}
	return append([]domain.MediaSearchResult(nil), result.([]domain.MediaSearchResult)...), nil
}

func (client *Client) fetchRelated(ctx context.Context, mediaType domain.MediaType, tmdbID int64) ([]domain.MediaSearchResult, error) {
	if err := client.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-client.requests }()
	results := make([]domain.MediaSearchResult, 0, 12)
	if mediaType == domain.MovieMediaType {
		var response *tmdbapi.MovieRecommendations
		if err := client.requestHeld(ctx, func() error {
			var requestErr error
			response, requestErr = client.client.GetMovieRecommendations(int(tmdbID), nil)
			return requestErr
		}); err != nil {
			return nil, err
		}
		if response.MovieRecommendationsResults == nil {
			return results, nil
		}
		for _, item := range response.Results {
			if item.Adult || item.ID <= 0 || item.ID == tmdbID || strings.TrimSpace(item.Title) == "" {
				continue
			}
			results = append(results, domain.MediaSearchResult{TMDBID: item.ID, Type: domain.MovieMediaType, Title: item.Title, OriginalTitle: item.OriginalTitle, Overview: item.Overview, ReleaseDate: item.ReleaseDate, PosterPath: item.PosterPath, OriginalLanguage: item.OriginalLanguage, BackdropPath: item.BackdropPath})
			if len(results) == 12 {
				break
			}
		}
		return results, nil
	}
	var response *tmdbapi.TVRecommendations
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		response, requestErr = client.client.GetTVRecommendations(int(tmdbID), nil)
		return requestErr
	}); err != nil {
		return nil, err
	}
	if response.TVRecommendationsResults == nil {
		return results, nil
	}
	for _, item := range response.Results {
		if item.ID <= 0 || item.ID == tmdbID || strings.TrimSpace(item.Name) == "" {
			continue
		}
		results = append(results, domain.MediaSearchResult{TMDBID: item.ID, Type: domain.TVMediaType, Title: item.Name, OriginalTitle: item.OriginalName, Overview: item.Overview, ReleaseDate: item.FirstAirDate, PosterPath: item.PosterPath, OriginalLanguage: item.OriginalLanguage, BackdropPath: item.BackdropPath})
		if len(results) == 12 {
			break
		}
	}
	return results, nil
}

func (client *Client) fetchTrending(ctx context.Context, mediaType, timeWindow string) ([]domain.MediaSearchResult, error) {
	if err := client.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-client.requests }()
	var response *tmdbapi.Trending
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		response, requestErr = client.client.GetTrending(mediaType, timeWindow, nil)
		return requestErr
	}); err != nil {
		return nil, err
	}
	results := make([]domain.MediaSearchResult, 0, len(response.Results))
	for _, item := range response.Results {
		title, originalTitle, releaseDate := item.Title, item.OriginalTitle, item.ReleaseDate
		if mediaType == "tv" {
			title, originalTitle, releaseDate = item.Name, item.OriginalName, item.FirstAirDate
		}
		results = append(results, domain.MediaSearchResult{TMDBID: item.ID, Type: domain.MediaType(mediaType), Title: title, OriginalTitle: originalTitle, Overview: item.Overview, ReleaseDate: releaseDate, PosterPath: item.PosterPath, OriginalLanguage: item.OriginalLanguage, BackdropPath: item.BackdropPath})
	}
	return results, nil
}

func (client *Client) search(ctx context.Context, query, language string) ([]domain.MediaSearchResult, error) {
	options := map[string]string{}
	if language = strings.TrimSpace(language); language != "" {
		options["language"] = language
	}
	var response *tmdbapi.SearchMulti
	err := client.request(ctx, func() error {
		var requestErr error
		response, requestErr = client.client.GetSearchMulti(query, options)
		return requestErr
	})
	if err != nil {
		return nil, err
	}

	results := make([]domain.MediaSearchResult, 0, len(response.Results))
	for _, item := range response.Results {
		mediaType := domain.MediaType(item.MediaType)
		if mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType {
			continue
		}
		title, originalTitle, releaseDate := item.Title, item.OriginalTitle, item.ReleaseDate
		if mediaType == domain.TVMediaType {
			title, originalTitle, releaseDate = item.Name, item.OriginalName, item.FirstAirDate
		}
		results = append(results, domain.MediaSearchResult{
			TMDBID:           item.ID,
			Type:             mediaType,
			Title:            title,
			OriginalTitle:    originalTitle,
			Overview:         item.Overview,
			ReleaseDate:      releaseDate,
			PosterPath:       item.PosterPath,
			OriginalLanguage: item.OriginalLanguage,
			BackdropPath:     item.BackdropPath,
		})
	}
	return results, nil
}

func (client *Client) Show(ctx context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	if tmdbID <= 0 {
		return domain.TVShowMetadata{}, fmt.Errorf("TMDB show ID must be positive")
	}
	client.mu.Lock()
	if cached, ok := client.showCache[tmdbID]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneShow(cached.show), nil
	}
	client.mu.Unlock()
	result, err := client.coalesce(ctx, fmt.Sprintf("show:%d", tmdbID), func() (any, error) {
		show, err := client.fetchShow(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.cacheShowLocked(tmdbID, show, time.Now().Add(showCacheTTL))
		client.mu.Unlock()
		return show, nil
	})
	if err != nil {
		return domain.TVShowMetadata{}, err
	}
	return cloneShow(result.(domain.TVShowMetadata)), nil
}

func (client *Client) ShowSummary(ctx context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	if tmdbID <= 0 {
		return domain.TVShowMetadata{}, fmt.Errorf("TMDB show ID must be positive")
	}
	client.mu.Lock()
	if cached, ok := client.showCache[tmdbID]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneShow(cached.show), nil
	}
	if cached, ok := client.summaryCache[tmdbID]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneShow(cached.show), nil
	}
	client.mu.Unlock()
	result, err := client.coalesce(ctx, fmt.Sprintf("show-summary:%d", tmdbID), func() (any, error) {
		show, err := client.fetchShowSummary(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.summaryCache[tmdbID] = cachedShow{show: cloneShow(show), expiresAt: time.Now().Add(showCacheTTL)}
		now := time.Now()
		for id, cached := range client.summaryCache {
			if !now.Before(cached.expiresAt) {
				delete(client.summaryCache, id)
			}
		}
		for len(client.summaryCache) > maxShowCacheEntries {
			for id := range client.summaryCache {
				if id != tmdbID {
					delete(client.summaryCache, id)
					break
				}
			}
		}
		client.mu.Unlock()
		return show, nil
	})
	if err != nil {
		return domain.TVShowMetadata{}, err
	}
	return cloneShow(result.(domain.TVShowMetadata)), nil
}

// RefreshShow fetches the show summary and its latest regular season only.
// This keeps scheduled refresh traffic bounded even for long-running shows.
func (client *Client) RefreshShow(ctx context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	show, err := client.ShowSummary(ctx, tmdbID)
	if err != nil {
		return domain.TVShowMetadata{}, err
	}
	latestSeason, found := latestRegularSeasonNumber(show.Seasons)
	if !found {
		return show, nil
	}
	season, err := client.Season(ctx, tmdbID, latestSeason)
	if err != nil {
		return domain.TVShowMetadata{}, err
	}
	for index := range show.Seasons {
		if show.Seasons[index].Number == latestSeason {
			show.Seasons[index].Episodes = season.Episodes
			break
		}
	}
	return show, nil
}

func latestRegularSeasonNumber(seasons []domain.TVSeasonMetadata) (int, bool) {
	latest, found := 0, false
	for _, season := range seasons {
		if season.Number > 0 && (!found || season.Number > latest) {
			latest, found = season.Number, true
		}
	}
	return latest, found
}

func (client *Client) Season(ctx context.Context, tmdbID int64, seasonNumber int) (domain.TVSeasonMetadata, error) {
	if tmdbID <= 0 || seasonNumber < 0 {
		return domain.TVSeasonMetadata{}, fmt.Errorf("TMDB show and season IDs must be valid")
	}
	cacheKey := fmt.Sprintf("%d:%d", tmdbID, seasonNumber)
	client.mu.Lock()
	if cached, ok := client.seasonCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneSeason(cached.season), nil
	}
	client.mu.Unlock()
	result, err := client.coalesce(ctx, "season:"+cacheKey, func() (any, error) {
		season, err := client.fetchSeason(ctx, tmdbID, seasonNumber)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.seasonCache[cacheKey] = cachedSeason{season: cloneSeason(season), expiresAt: time.Now().Add(showCacheTTL)}
		now := time.Now()
		for key, cached := range client.seasonCache {
			if !now.Before(cached.expiresAt) {
				delete(client.seasonCache, key)
			}
		}
		for len(client.seasonCache) > maxShowCacheEntries*10 {
			for key := range client.seasonCache {
				if key != cacheKey {
					delete(client.seasonCache, key)
					break
				}
			}
		}
		client.mu.Unlock()
		return season, nil
	})
	if err != nil {
		return domain.TVSeasonMetadata{}, err
	}
	return cloneSeason(result.(domain.TVSeasonMetadata)), nil
}

func (client *Client) Movie(ctx context.Context, tmdbID int64) (domain.MovieMetadata, error) {
	if tmdbID <= 0 {
		return domain.MovieMetadata{}, fmt.Errorf("TMDB movie ID must be positive")
	}
	client.mu.Lock()
	if cached, ok := client.movieCache[tmdbID]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneMovie(cached.movie), nil
	}
	client.mu.Unlock()
	result, err := client.coalesce(ctx, fmt.Sprintf("movie:%d", tmdbID), func() (any, error) {
		movie, err := client.fetchMovie(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.movieCache[tmdbID] = cachedMovie{movie: cloneMovie(movie), expiresAt: time.Now().Add(showCacheTTL)}
		client.mu.Unlock()
		return movie, nil
	})
	if err != nil {
		return domain.MovieMetadata{}, err
	}
	return cloneMovie(result.(domain.MovieMetadata)), nil
}

func (client *Client) Person(ctx context.Context, tmdbID int64) (domain.PersonMetadata, error) {
	if tmdbID <= 0 {
		return domain.PersonMetadata{}, fmt.Errorf("TMDB person ID must be positive")
	}
	client.mu.Lock()
	if cached, ok := client.personCache[tmdbID]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return clonePerson(cached.person), nil
	}
	client.mu.Unlock()
	result, err := client.coalesce(ctx, fmt.Sprintf("person:%d", tmdbID), func() (any, error) {
		person, err := client.fetchPerson(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.personCache[tmdbID] = cachedPerson{person: clonePerson(person), expiresAt: time.Now().Add(showCacheTTL)}
		now := time.Now()
		for id, cached := range client.personCache {
			if !now.Before(cached.expiresAt) {
				delete(client.personCache, id)
			}
		}
		for len(client.personCache) > maxShowCacheEntries {
			for id := range client.personCache {
				if id != tmdbID {
					delete(client.personCache, id)
					break
				}
			}
		}
		client.mu.Unlock()
		return person, nil
	})
	if err != nil {
		return domain.PersonMetadata{}, err
	}
	return clonePerson(result.(domain.PersonMetadata)), nil
}

func (client *Client) Episode(ctx context.Context, showID int64, seasonNumber int, episodeNumber int) (domain.TVEpisodeMetadata, error) {
	if showID <= 0 || seasonNumber < 0 || episodeNumber <= 0 {
		return domain.TVEpisodeMetadata{}, fmt.Errorf("TMDB show, season, and episode must be valid")
	}
	cacheKey := fmt.Sprintf("%d:%d:%d", showID, seasonNumber, episodeNumber)
	client.mu.Lock()
	if cached, ok := client.episodeCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneEpisode(cached.episode), nil
	}
	client.mu.Unlock()
	result, err := client.coalesce(ctx, "episode:"+cacheKey, func() (any, error) {
		episode, err := client.fetchEpisode(ctx, showID, seasonNumber, episodeNumber)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.episodeCache[cacheKey] = cachedEpisode{episode: cloneEpisode(episode), expiresAt: time.Now().Add(showCacheTTL)}
		client.mu.Unlock()
		return episode, nil
	})
	if err != nil {
		return domain.TVEpisodeMetadata{}, err
	}
	return cloneEpisode(result.(domain.TVEpisodeMetadata)), nil
}

func (client *Client) coalesce(ctx context.Context, key string, fetch func() (any, error)) (any, error) {
	result := client.calls.DoChan(key, fetch)
	select {
	case value := <-result:
		return value.Val, value.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// cacheShowLocked bounds retained show metadata in addition to its TTL. Search
// and show results can be large, so arbitrary browsing must not grow memory
// without limit. The caller must hold client.mu.
func (client *Client) cacheShowLocked(tmdbID int64, show domain.TVShowMetadata, expiresAt time.Time) {
	client.showCache[tmdbID] = cachedShow{show: cloneShow(show), expiresAt: expiresAt}
	now := time.Now()
	for id, cached := range client.showCache {
		if !now.Before(cached.expiresAt) {
			delete(client.showCache, id)
		}
	}
	for len(client.showCache) > maxShowCacheEntries {
		for id := range client.showCache {
			if id != tmdbID {
				delete(client.showCache, id)
				break
			}
		}
	}
}

func cloneShow(show domain.TVShowMetadata) domain.TVShowMetadata {
	clone := show
	clone.Seasons = make([]domain.TVSeasonMetadata, len(show.Seasons))
	for index, season := range show.Seasons {
		clone.Seasons[index] = season
		clone.Seasons[index].Episodes = append([]domain.TVEpisodeMetadata(nil), season.Episodes...)
	}
	clone.Cast = append([]domain.TVCastMember(nil), show.Cast...)
	return clone
}

func cloneSeason(season domain.TVSeasonMetadata) domain.TVSeasonMetadata {
	clone := season
	clone.Episodes = append([]domain.TVEpisodeMetadata(nil), season.Episodes...)
	return clone
}

func cloneEpisode(episode domain.TVEpisodeMetadata) domain.TVEpisodeMetadata {
	clone := episode
	clone.GuestStars = append([]domain.TVCastMember(nil), episode.GuestStars...)
	clone.Crew = append([]domain.TVCrewMember(nil), episode.Crew...)
	return clone
}

func cloneMovie(movie domain.MovieMetadata) domain.MovieMetadata {
	clone := movie
	clone.Genres = append([]string(nil), movie.Genres...)
	clone.Cast = append([]domain.TVCastMember(nil), movie.Cast...)
	return clone
}

func clonePerson(person domain.PersonMetadata) domain.PersonMetadata {
	clone := person
	clone.Credits = append([]domain.PersonCredit(nil), person.Credits...)
	return clone
}

func (client *Client) fetchShow(ctx context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	show, err := client.ShowSummary(ctx, tmdbID)
	if err != nil {
		return domain.TVShowMetadata{}, err
	}
	for index := range show.Seasons {
		season, err := client.Season(ctx, tmdbID, show.Seasons[index].Number)
		if err != nil {
			return domain.TVShowMetadata{}, err
		}
		show.Seasons[index].Episodes = season.Episodes
	}
	return show, nil
}

func (client *Client) fetchShowSummary(ctx context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	if err := client.acquire(ctx); err != nil {
		return domain.TVShowMetadata{}, err
	}
	defer func() { <-client.requests }()
	var details *tmdbapi.TVDetails
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		details, requestErr = client.client.GetTVDetails(int(tmdbID), map[string]string{"append_to_response": "credits"})
		return requestErr
	}); err != nil {
		return domain.TVShowMetadata{}, err
	}
	show := domain.TVShowMetadata{TMDBID: details.ID, Name: details.Name, Overview: details.Overview, PosterPath: details.PosterPath, BackdropPath: details.BackdropPath, FirstAirDate: details.FirstAirDate, OriginalLanguage: details.OriginalLanguage, Status: details.Status}
	if details.TVCreditsAppend != nil && details.Credits.TVCredits != nil {
		for _, member := range details.Credits.Cast {
			show.Cast = append(show.Cast, domain.TVCastMember{ID: member.ID, Name: member.Name, Character: member.Character, ProfilePath: member.ProfilePath})
		}
	}
	for _, season := range details.Seasons {
		show.Seasons = append(show.Seasons, domain.TVSeasonMetadata{TMDBID: season.ID, Number: season.SeasonNumber, Name: season.Name, Overview: season.Overview, PosterPath: season.PosterPath, AirDate: season.AirDate})
	}
	return show, nil
}

func (client *Client) fetchSeason(ctx context.Context, tmdbID int64, seasonNumber int) (domain.TVSeasonMetadata, error) {
	if err := client.acquire(ctx); err != nil {
		return domain.TVSeasonMetadata{}, err
	}
	defer func() { <-client.requests }()
	var seasonDetails *tmdbapi.TVSeasonDetails
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		seasonDetails, requestErr = client.client.GetTVSeasonDetails(int(tmdbID), seasonNumber, nil)
		return requestErr
	}); err != nil {
		return domain.TVSeasonMetadata{}, err
	}
	season := domain.TVSeasonMetadata{TMDBID: seasonDetails.ID, Number: seasonDetails.SeasonNumber, Name: seasonDetails.Name, Overview: seasonDetails.Overview, PosterPath: seasonDetails.PosterPath, AirDate: seasonDetails.AirDate}
	for _, episode := range seasonDetails.Episodes {
		season.Episodes = append(season.Episodes, domain.TVEpisodeMetadata{TMDBID: episode.ID, SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber, Name: episode.Name, Overview: episode.Overview, AirDate: episode.AirDate, Runtime: episode.Runtime, StillPath: episode.StillPath})
	}
	return season, nil
}

func (client *Client) fetchMovie(ctx context.Context, tmdbID int64) (domain.MovieMetadata, error) {
	if err := client.acquire(ctx); err != nil {
		return domain.MovieMetadata{}, err
	}
	defer func() { <-client.requests }()
	var details *tmdbapi.MovieDetails
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		details, requestErr = client.client.GetMovieDetails(int(tmdbID), map[string]string{"append_to_response": "credits"})
		return requestErr
	}); err != nil {
		return domain.MovieMetadata{}, err
	}
	movie := domain.MovieMetadata{TMDBID: details.ID, Title: details.Title, OriginalTitle: details.OriginalTitle, Overview: details.Overview, PosterPath: details.PosterPath, BackdropPath: details.BackdropPath, ReleaseDate: details.ReleaseDate, OriginalLanguage: details.OriginalLanguage, Runtime: details.Runtime, Status: details.Status, VoteAverage: details.VoteAverage}
	for _, genre := range details.Genres {
		movie.Genres = append(movie.Genres, genre.Name)
	}
	if details.MovieCreditsAppend != nil && details.Credits.MovieCredits != nil {
		for _, member := range details.Credits.Cast {
			movie.Cast = append(movie.Cast, domain.TVCastMember{ID: member.ID, Name: member.Name, Character: member.Character, ProfilePath: member.ProfilePath})
		}
	}
	return movie, nil
}

func (client *Client) fetchPerson(ctx context.Context, tmdbID int64) (domain.PersonMetadata, error) {
	if err := client.acquire(ctx); err != nil {
		return domain.PersonMetadata{}, err
	}
	defer func() { <-client.requests }()
	var details *tmdbapi.PersonDetails
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		details, requestErr = client.client.GetPersonDetails(int(tmdbID), map[string]string{"append_to_response": "combined_credits"})
		return requestErr
	}); err != nil {
		return domain.PersonMetadata{}, err
	}
	person := domain.PersonMetadata{TMDBID: details.ID, Name: details.Name, Biography: details.Biography, ProfilePath: details.ProfilePath, Birthday: details.Birthday, Deathday: details.Deathday, PlaceOfBirth: details.PlaceOfBirth, KnownFor: details.KnownForDepartment, Credits: []domain.PersonCredit{}}
	seen := make(map[string]int)
	if details.PersonCombinedCreditsAppend != nil && details.CombinedCredits != nil {
		for _, credit := range details.CombinedCredits.Cast {
			mediaType := domain.MediaType(credit.MediaType)
			if (mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType) || credit.Adult || credit.ID <= 0 {
				continue
			}
			title, originalTitle, releaseDate := credit.Title, credit.OriginalTitle, credit.ReleaseDate
			if mediaType == domain.TVMediaType {
				title, originalTitle, releaseDate = credit.Name, credit.OriginalName, credit.FirstAirDate
			}
			if title == "" {
				continue
			}
			item := domain.PersonCredit{TMDBID: credit.ID, Type: mediaType, Title: title, OriginalTitle: originalTitle, Character: credit.Character, Overview: credit.Overview, ReleaseDate: releaseDate, PosterPath: credit.PosterPath, BackdropPath: credit.BackdropPath, OriginalLanguage: credit.OriginalLanguage, Popularity: credit.Popularity}
			key := fmt.Sprintf("%s:%d", mediaType, credit.ID)
			if index, exists := seen[key]; exists {
				if person.Credits[index].Character == "" && item.Character != "" {
					person.Credits[index].Character = item.Character
				}
				continue
			}
			seen[key] = len(person.Credits)
			person.Credits = append(person.Credits, item)
		}
	}
	sort.SliceStable(person.Credits, func(i, j int) bool {
		if person.Credits[i].Popularity != person.Credits[j].Popularity {
			return person.Credits[i].Popularity > person.Credits[j].Popularity
		}
		return person.Credits[i].Title < person.Credits[j].Title
	})
	return person, nil
}

func (client *Client) fetchEpisode(ctx context.Context, showID int64, seasonNumber int, episodeNumber int) (domain.TVEpisodeMetadata, error) {
	if err := client.acquire(ctx); err != nil {
		return domain.TVEpisodeMetadata{}, err
	}
	defer func() { <-client.requests }()
	var details *tmdbapi.TVEpisodeDetails
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		details, requestErr = client.client.GetTVEpisodeDetails(int(showID), seasonNumber, episodeNumber, map[string]string{"append_to_response": "credits"})
		return requestErr
	}); err != nil {
		return domain.TVEpisodeMetadata{}, err
	}
	episode := domain.TVEpisodeMetadata{TMDBID: details.ID, SeasonNumber: details.SeasonNumber, EpisodeNumber: details.EpisodeNumber, Name: details.Name, Overview: details.Overview, AirDate: details.AirDate, Runtime: details.Runtime, StillPath: details.StillPath, VoteAverage: details.VoteAverage, ProductionCode: details.ProductionCode}
	for _, member := range details.GuestStars {
		episode.GuestStars = append(episode.GuestStars, domain.TVCastMember{ID: member.ID, Name: member.Name, Character: member.Character, ProfilePath: member.ProfilePath})
	}
	if details.TVEpisodeCreditsAppend != nil && details.Credits != nil {
		for _, member := range details.Credits.Crew {
			episode.Crew = append(episode.Crew, domain.TVCrewMember{ID: member.ID, Name: member.Name, Job: member.Job, Department: member.Department, ProfilePath: member.ProfilePath})
		}
	}
	return episode, nil
}

func (client *Client) request(ctx context.Context, operation func() error) error {
	if err := client.acquire(ctx); err != nil {
		return err
	}
	defer func() { <-client.requests }()
	return client.requestHeld(ctx, operation)
}

func (client *Client) requestHeld(ctx context.Context, operation func() error) error {
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err = client.waitForProvider(ctx); err != nil {
			return err
		}
		err = operation()
		if err == nil {
			return nil
		}
		if !isRateLimited(err) {
			return mapError(err)
		}
		delay := time.Duration(1<<attempt)*time.Second + time.Duration(rand.Intn(251))*time.Millisecond
		client.mu.Lock()
		retryAfter := client.retryAfter
		client.retryAfter = 0
		client.mu.Unlock()
		if retryAfter > delay {
			delay = retryAfter
		}
		client.pause(delay)
	}
	return mapError(err)
}

func (client *Client) captureRetryAfter(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	delay := time.Duration(0)
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds > 0 {
		delay = time.Duration(seconds) * time.Second
	} else if until, err := http.ParseTime(value); err == nil {
		delay = time.Until(until)
	}
	if delay <= 0 {
		return
	}
	client.mu.Lock()
	if delay > client.retryAfter {
		client.retryAfter = delay
	}
	client.mu.Unlock()
}

func (client *Client) acquire(ctx context.Context) error {
	select {
	case client.requests <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (client *Client) waitForProvider(ctx context.Context) error {
	client.mu.Lock()
	pausedUntil := client.pausedUntil
	client.mu.Unlock()
	if delay := time.Until(pausedUntil); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (client *Client) pause(delay time.Duration) {
	until := time.Now().Add(delay)
	client.mu.Lock()
	if until.After(client.pausedUntil) {
		client.pausedUntil = until
	}
	client.mu.Unlock()
}

func isRateLimited(err error) bool {
	var providerError tmdbapi.Error
	return errors.As(err, &providerError) && providerError.StatusCode == http.StatusTooManyRequests
}

func mapError(err error) error {
	if isRateLimited(err) {
		return fmt.Errorf("%w: TMDB rate limit reached", ErrTemporarilyUnavailable)
	}
	return fmt.Errorf("TMDB request: %w", err)
}

var ErrTemporarilyUnavailable = errors.New("metadata temporarily unavailable")

var _ domain.MetadataProvider = (*Client)(nil)
var _ domain.TVShowMetadataProvider = (*Client)(nil)
var _ domain.ScheduledTVShowMetadataProvider = (*Client)(nil)
var _ domain.PersonMetadataProvider = (*Client)(nil)
var _ domain.RelatedMetadataProvider = (*Client)(nil)
