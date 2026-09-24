package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	tmdbapi "github.com/cyruzin/golang-tmdb"
)

const defaultRequestTimeout = 10 * time.Second

// Client is Visto's TMDB adapter. The third-party client is deliberately kept
// inside this package, so it cannot leak into application or domain code.
type Client struct {
	client *tmdbapi.Client
}

func New(apiKey string, httpClient *http.Client) (*Client, error) {
	client, err := tmdbapi.Init(apiKey)
	if err != nil {
		return nil, fmt.Errorf("initialize TMDB client: %w", err)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout}
	}
	if httpClient.Timeout <= 0 {
		httpClient.Timeout = defaultRequestTimeout
	}
	client.SetClientConfig(*httpClient)
	return &Client{client: client}, nil
}

func (client *Client) Search(ctx context.Context, query, language string) ([]domain.MediaSearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []domain.MediaSearchResult{}, nil
	}

	options := map[string]string{}
	if language = strings.TrimSpace(language); language != "" {
		options["language"] = language
	}
	response, err := client.client.GetSearchMulti(query, options)
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
	return results, nil
}

func mapError(err error) error {
	var providerError tmdbapi.Error
	if errors.As(err, &providerError) && providerError.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("%w: TMDB rate limit reached", ErrTemporarilyUnavailable)
	}
	return fmt.Errorf("TMDB request: %w", err)
}

var ErrTemporarilyUnavailable = errors.New("metadata temporarily unavailable")

var _ domain.MetadataProvider = (*Client)(nil)
