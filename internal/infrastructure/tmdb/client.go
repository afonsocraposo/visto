package tmdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	tmdbapi "github.com/cyruzin/golang-tmdb"
)

const (
	defaultRequestTimeout = 10 * time.Second
	searchCacheTTL        = 5 * time.Minute
	showCacheTTL          = 24 * time.Hour
	maxAttempts           = 3
	maxSearchCacheEntries = 500
	maxShowCacheEntries   = 100
)

// Client is Visto's TMDB adapter. The third-party client is deliberately kept
// inside this package, so it cannot leak into application or domain code.
type Client struct {
	client      *tmdbapi.Client
	requests    chan struct{}
	mu          sync.Mutex
	pausedUntil time.Time
	retryAfter  time.Duration
	searchCache map[string]cachedSearch
	searchCalls map[string]*searchCall
	showCache   map[int64]cachedShow
	showCalls   map[int64]*showCall
}

type cachedSearch struct {
	results   []domain.MediaSearchResult
	expiresAt time.Time
}

type searchCall struct {
	done    chan struct{}
	results []domain.MediaSearchResult
	err     error
}
type cachedShow struct {
	show      domain.TVShowMetadata
	expiresAt time.Time
}
type showCall struct {
	done chan struct{}
	show domain.TVShowMetadata
	err  error
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
	vistoClient := &Client{client: client, requests: make(chan struct{}, 4), searchCache: map[string]cachedSearch{}, searchCalls: map[string]*searchCall{}, showCache: map[int64]cachedShow{}, showCalls: map[int64]*showCall{}}
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
	if call, ok := client.searchCalls[cacheKey]; ok {
		client.mu.Unlock()
		select {
		case <-call.done:
			return append([]domain.MediaSearchResult(nil), call.results...), call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &searchCall{done: make(chan struct{})}
	client.searchCalls[cacheKey] = call
	client.mu.Unlock()
	results, err := client.search(ctx, query, language)
	client.mu.Lock()
	call.results = append([]domain.MediaSearchResult(nil), results...)
	call.err = err
	if err == nil {
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
	}
	delete(client.searchCalls, cacheKey)
	close(call.done)
	client.mu.Unlock()
	return results, err
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
	if call, ok := client.showCalls[tmdbID]; ok {
		client.mu.Unlock()
		select {
		case <-call.done:
			return cloneShow(call.show), call.err
		case <-ctx.Done():
			return domain.TVShowMetadata{}, ctx.Err()
		}
	}
	call := &showCall{done: make(chan struct{})}
	client.showCalls[tmdbID] = call
	client.mu.Unlock()
	show, err := client.fetchShow(ctx, tmdbID)
	client.mu.Lock()
	call.show = cloneShow(show)
	call.err = err
	if err == nil {
		client.cacheShowLocked(tmdbID, show, time.Now().Add(showCacheTTL))
	}
	delete(client.showCalls, tmdbID)
	close(call.done)
	client.mu.Unlock()
	return cloneShow(show), err
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
	return clone
}

func (client *Client) fetchShow(ctx context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	if err := client.acquire(ctx); err != nil {
		return domain.TVShowMetadata{}, err
	}
	defer func() { <-client.requests }()
	showID := int(tmdbID)
	var details *tmdbapi.TVDetails
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		details, requestErr = client.client.GetTVDetails(showID, nil)
		return requestErr
	}); err != nil {
		return domain.TVShowMetadata{}, err
	}
	show := domain.TVShowMetadata{TMDBID: details.ID, Name: details.Name, Overview: details.Overview, PosterPath: details.PosterPath, FirstAirDate: details.FirstAirDate, OriginalLanguage: details.OriginalLanguage}
	for _, season := range details.Seasons {
		seasonMetadata := domain.TVSeasonMetadata{TMDBID: season.ID, Number: season.SeasonNumber, Name: season.Name, Overview: season.Overview, PosterPath: season.PosterPath, AirDate: season.AirDate}
		var seasonDetails *tmdbapi.TVSeasonDetails
		if err := client.requestHeld(ctx, func() error {
			var requestErr error
			seasonDetails, requestErr = client.client.GetTVSeasonDetails(showID, season.SeasonNumber, nil)
			return requestErr
		}); err != nil {
			return domain.TVShowMetadata{}, err
		}
		for _, episode := range seasonDetails.Episodes {
			seasonMetadata.Episodes = append(seasonMetadata.Episodes, domain.TVEpisodeMetadata{TMDBID: episode.ID, SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber, Name: episode.Name, Overview: episode.Overview, AirDate: episode.AirDate, Runtime: episode.Runtime, StillPath: episode.StillPath})
		}
		show.Seasons = append(show.Seasons, seasonMetadata)
	}
	return show, nil
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
