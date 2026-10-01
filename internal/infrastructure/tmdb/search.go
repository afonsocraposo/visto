package tmdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	tmdbapi "github.com/cyruzin/golang-tmdb"
)

// FindByTVDB resolves a Bingers external ID without fetching a full catalog.
func (client *Client) FindByTVDB(ctx context.Context, kind, tvdbID string) (int64, error) {
	if kind != "tv" && kind != "movie" {
		return 0, fmt.Errorf("invalid media type")
	}
	id, err := strconv.ParseInt(tvdbID, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid TVDB ID")
	}
	if err := client.acquire(ctx); err != nil {
		return 0, err
	}
	defer func() { <-client.requests }()
	var found *tmdbapi.FindByID
	err = client.requestHeld(ctx, func() error {
		var requestErr error
		found, requestErr = client.client.GetFindByID(tvdbID, map[string]string{"external_source": "tvdb_id"})
		return requestErr
	})
	if err != nil {
		return 0, err
	}
	if kind == "tv" && len(found.TvResults) == 1 {
		return found.TvResults[0].ID, nil
	}
	if kind == "movie" && len(found.MovieResults) == 1 {
		return found.MovieResults[0].ID, nil
	}
	return 0, fmt.Errorf("no unique TMDB match")
}

// FindEpisodeByExternalID locates an episode from an external episode ID ("tvdb_id" or
// "imdb_id"), which names the exact episode regardless of how a media server titles its show.
func (client *Client) FindEpisodeByExternalID(ctx context.Context, source, externalID string) (domain.EpisodeLocation, error) {
	if source != "tvdb_id" && source != "imdb_id" {
		return domain.EpisodeLocation{}, fmt.Errorf("unsupported external ID source")
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" || strings.ContainsAny(externalID, "/?&# ") {
		return domain.EpisodeLocation{}, fmt.Errorf("invalid external episode ID")
	}
	if err := client.acquire(ctx); err != nil {
		return domain.EpisodeLocation{}, err
	}
	defer func() { <-client.requests }()
	var found *tmdbapi.FindByID
	err := client.requestHeld(ctx, func() error {
		var requestErr error
		found, requestErr = client.client.GetFindByID(externalID, map[string]string{"external_source": source})
		return requestErr
	})
	if err != nil {
		return domain.EpisodeLocation{}, err
	}
	if len(found.TvEpisodeResults) != 1 || found.TvEpisodeResults[0].ShowID <= 0 {
		return domain.EpisodeLocation{}, fmt.Errorf("no unique TMDB episode match")
	}
	episode := found.TvEpisodeResults[0]
	return domain.EpisodeLocation{ShowTMDBID: episode.ShowID, SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber}, nil
}

func (client *Client) Search(ctx context.Context, query, language string) ([]domain.MediaSearchResult, error) {
	return client.cachedSearch(ctx, "", query, language, client.search)
}

// SearchType searches a single media type, so a filtered Discover search returns a full
// page of that type instead of whatever share of a mixed multi-search survives filtering.
func (client *Client) SearchType(ctx context.Context, mediaType domain.MediaType, query, language string) ([]domain.MediaSearchResult, error) {
	switch mediaType {
	case domain.TVMediaType:
		return client.cachedSearch(ctx, "tv", query, language, client.searchTV)
	case domain.MovieMediaType:
		return client.cachedSearch(ctx, "movie", query, language, client.searchMovies)
	default:
		return nil, fmt.Errorf("search media type must be movie or tv")
	}
}

func (client *Client) cachedSearch(ctx context.Context, kind, query, language string, fetch func(context.Context, string, string) ([]domain.MediaSearchResult, error)) ([]domain.MediaSearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	language = strings.TrimSpace(language)
	if query == "" {
		return []domain.MediaSearchResult{}, nil
	}
	cacheKey := language + "\x00" + query
	if kind != "" {
		cacheKey = kind + "\x00" + cacheKey
	}
	client.mu.Lock()
	if cached, ok := client.searchCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneSlice(cached.results), nil
	}
	client.mu.Unlock()

	result, err := client.coalesce(ctx, "search:"+cacheKey, func() (any, error) {
		results, err := fetch(ctx, query, language)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.searchCache[cacheKey] = cachedSearch{results: cloneSlice(results), expiresAt: time.Now().Add(searchCacheTTL)}
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
	return cloneSlice(result.([]domain.MediaSearchResult)), nil
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
		return cloneSlice(cached.results), nil
	}
	client.mu.Unlock()

	result, err := client.coalesce(ctx, "trending:"+cacheKey, func() (any, error) {
		results, err := client.fetchTrending(ctx, mediaType, timeWindow)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.searchCache[cacheKey] = cachedSearch{results: cloneSlice(results), expiresAt: time.Now().Add(searchCacheTTL)}
		client.mu.Unlock()
		return results, nil
	})
	if err != nil {
		return nil, err
	}
	return cloneSlice(result.([]domain.MediaSearchResult)), nil
}

func (client *Client) Related(ctx context.Context, mediaType domain.MediaType, tmdbID int64) ([]domain.MediaSearchResult, error) {
	if (mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType) || tmdbID <= 0 {
		return nil, fmt.Errorf("related media type and TMDB ID must be valid")
	}
	cacheKey := fmt.Sprintf("%s:%d", mediaType, tmdbID)
	client.mu.Lock()
	if cached, ok := client.relatedCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneSlice(cached.results), nil
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
		client.relatedCache[cacheKey] = cachedSearch{results: cloneSlice(results), expiresAt: now.Add(relatedCacheTTL)}
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
	return cloneSlice(result.([]domain.MediaSearchResult)), nil
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

func searchOptions(language string) map[string]string {
	options := map[string]string{}
	if language != "" {
		options["language"] = language
	}
	return options
}

func (client *Client) searchTV(ctx context.Context, query, language string) ([]domain.MediaSearchResult, error) {
	var response *tmdbapi.SearchTVShows
	err := client.request(ctx, func() error {
		var requestErr error
		response, requestErr = client.client.GetSearchTVShow(query, searchOptions(language))
		return requestErr
	})
	if err != nil {
		return nil, err
	}
	results := []domain.MediaSearchResult{}
	if response.SearchTVShowsResults == nil {
		return results, nil
	}
	for _, item := range response.Results {
		results = append(results, domain.MediaSearchResult{
			TMDBID:           item.ID,
			Type:             domain.TVMediaType,
			Title:            item.Name,
			OriginalTitle:    item.OriginalName,
			Overview:         item.Overview,
			ReleaseDate:      item.FirstAirDate,
			PosterPath:       item.PosterPath,
			OriginalLanguage: item.OriginalLanguage,
			BackdropPath:     item.BackdropPath,
		})
	}
	return results, nil
}

func (client *Client) searchMovies(ctx context.Context, query, language string) ([]domain.MediaSearchResult, error) {
	var response *tmdbapi.SearchMovies
	err := client.request(ctx, func() error {
		var requestErr error
		response, requestErr = client.client.GetSearchMovies(query, searchOptions(language))
		return requestErr
	})
	if err != nil {
		return nil, err
	}
	results := []domain.MediaSearchResult{}
	if response.SearchMoviesResults == nil {
		return results, nil
	}
	for _, item := range response.Results {
		results = append(results, domain.MediaSearchResult{
			TMDBID:           item.ID,
			Type:             domain.MovieMediaType,
			Title:            item.Title,
			OriginalTitle:    item.OriginalTitle,
			Overview:         item.Overview,
			ReleaseDate:      item.ReleaseDate,
			PosterPath:       item.PosterPath,
			OriginalLanguage: item.OriginalLanguage,
			BackdropPath:     item.BackdropPath,
		})
	}
	return results, nil
}
