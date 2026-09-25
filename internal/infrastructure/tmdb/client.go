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

func (client *Client) coalesce(ctx context.Context, key string, fetch func() (any, error)) (any, error) {
	result := client.calls.DoChan(key, fetch)
	select {
	case value := <-result:
		return value.Val, value.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
