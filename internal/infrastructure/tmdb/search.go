package tmdb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	tmdbapi "github.com/cyruzin/golang-tmdb"
)

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
