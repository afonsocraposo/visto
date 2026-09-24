package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	tmdbapi "github.com/cyruzin/golang-tmdb"
)

const (
	defaultRequestTimeout = 10 * time.Second
	searchCacheTTL        = 5 * time.Minute
	maxAttempts           = 3
)

// Client is Visto's TMDB adapter. The third-party client is deliberately kept
// inside this package, so it cannot leak into application or domain code.
type Client struct {
	client      *tmdbapi.Client
	requests    chan struct{}
	mu          sync.Mutex
	pausedUntil time.Time
	searchCache map[string]cachedSearch
}

type cachedSearch struct {
	results   []domain.MediaSearchResult
	expiresAt time.Time
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
	client.SetClientConfig(*httpClient)
	return &Client{client: client, requests: make(chan struct{}, 4), searchCache: map[string]cachedSearch{}}, nil
}

func (client *Client) Search(ctx context.Context, query, language string) ([]domain.MediaSearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
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

	options := map[string]string{}
	if language = strings.TrimSpace(language); language != "" {
		options["language"] = language
	}
	if err := client.acquire(ctx); err != nil {
		return nil, err
	}
	defer func() { <-client.requests }()
	var response *tmdbapi.SearchMulti
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err = client.waitForProvider(ctx); err != nil {
			return nil, err
		}
		response, err = client.client.GetSearchMulti(query, options)
		if err == nil {
			break
		}
		if !isRateLimited(err) {
			return nil, mapError(err)
		}
		client.pause(time.Duration(1<<attempt) * time.Second)
	}
	if err != nil {
		return nil, mapError(err)
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
	client.mu.Lock()
	client.searchCache[cacheKey] = cachedSearch{results: append([]domain.MediaSearchResult(nil), results...), expiresAt: time.Now().Add(searchCacheTTL)}
	client.mu.Unlock()
	return results, nil
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
