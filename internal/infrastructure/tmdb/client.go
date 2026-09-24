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
	client       *tmdbapi.Client
	requests     chan struct{}
	mu           sync.Mutex
	pausedUntil  time.Time
	retryAfter   time.Duration
	searchCache  map[string]cachedSearch
	searchCalls  map[string]*searchCall
	showCache    map[int64]cachedShow
	showCalls    map[int64]*showCall
	summaryCache map[int64]cachedShow
	summaryCalls map[int64]*showCall
	seasonCache  map[string]cachedSeason
	seasonCalls  map[string]*seasonCall
	movieCache   map[int64]cachedMovie
	movieCalls   map[int64]*movieCall
	episodeCache map[string]cachedEpisode
	episodeCalls map[string]*episodeCall
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
type cachedSeason struct {
	season    domain.TVSeasonMetadata
	expiresAt time.Time
}
type seasonCall struct {
	done   chan struct{}
	season domain.TVSeasonMetadata
	err    error
}
type cachedMovie struct {
	movie     domain.MovieMetadata
	expiresAt time.Time
}
type movieCall struct {
	done  chan struct{}
	movie domain.MovieMetadata
	err   error
}
type cachedEpisode struct {
	episode   domain.TVEpisodeMetadata
	expiresAt time.Time
}
type episodeCall struct {
	done    chan struct{}
	episode domain.TVEpisodeMetadata
	err     error
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
	vistoClient := &Client{client: client, requests: make(chan struct{}, 4), searchCache: map[string]cachedSearch{}, searchCalls: map[string]*searchCall{}, showCache: map[int64]cachedShow{}, showCalls: map[int64]*showCall{}, summaryCache: map[int64]cachedShow{}, summaryCalls: map[int64]*showCall{}, seasonCache: map[string]cachedSeason{}, seasonCalls: map[string]*seasonCall{}, movieCache: map[int64]cachedMovie{}, movieCalls: map[int64]*movieCall{}, episodeCache: map[string]cachedEpisode{}, episodeCalls: map[string]*episodeCall{}}
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

	results, err := client.fetchTrending(ctx, mediaType, timeWindow)
	client.mu.Lock()
	call.results = append([]domain.MediaSearchResult(nil), results...)
	call.err = err
	if err == nil {
		client.searchCache[cacheKey] = cachedSearch{results: append([]domain.MediaSearchResult(nil), results...), expiresAt: time.Now().Add(searchCacheTTL)}
	}
	delete(client.searchCalls, cacheKey)
	close(call.done)
	client.mu.Unlock()
	return results, err
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
	if call, ok := client.summaryCalls[tmdbID]; ok {
		client.mu.Unlock()
		select {
		case <-call.done:
			return cloneShow(call.show), call.err
		case <-ctx.Done():
			return domain.TVShowMetadata{}, ctx.Err()
		}
	}
	call := &showCall{done: make(chan struct{})}
	client.summaryCalls[tmdbID] = call
	client.mu.Unlock()
	show, err := client.fetchShowSummary(ctx, tmdbID)
	client.mu.Lock()
	call.show = cloneShow(show)
	call.err = err
	if err == nil {
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
	}
	delete(client.summaryCalls, tmdbID)
	close(call.done)
	client.mu.Unlock()
	return cloneShow(show), err
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
	if call, ok := client.seasonCalls[cacheKey]; ok {
		client.mu.Unlock()
		select {
		case <-call.done:
			return cloneSeason(call.season), call.err
		case <-ctx.Done():
			return domain.TVSeasonMetadata{}, ctx.Err()
		}
	}
	call := &seasonCall{done: make(chan struct{})}
	client.seasonCalls[cacheKey] = call
	client.mu.Unlock()
	season, err := client.fetchSeason(ctx, tmdbID, seasonNumber)
	client.mu.Lock()
	call.season = cloneSeason(season)
	call.err = err
	if err == nil {
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
	}
	delete(client.seasonCalls, cacheKey)
	close(call.done)
	client.mu.Unlock()
	return cloneSeason(season), err
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
	if call, ok := client.movieCalls[tmdbID]; ok {
		client.mu.Unlock()
		select {
		case <-call.done:
			return cloneMovie(call.movie), call.err
		case <-ctx.Done():
			return domain.MovieMetadata{}, ctx.Err()
		}
	}
	call := &movieCall{done: make(chan struct{})}
	client.movieCalls[tmdbID] = call
	client.mu.Unlock()
	movie, err := client.fetchMovie(ctx, tmdbID)
	client.mu.Lock()
	call.movie = cloneMovie(movie)
	call.err = err
	if err == nil {
		client.movieCache[tmdbID] = cachedMovie{movie: cloneMovie(movie), expiresAt: time.Now().Add(showCacheTTL)}
	}
	delete(client.movieCalls, tmdbID)
	close(call.done)
	client.mu.Unlock()
	return cloneMovie(movie), err
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
	if call, ok := client.episodeCalls[cacheKey]; ok {
		client.mu.Unlock()
		select {
		case <-call.done:
			return cloneEpisode(call.episode), call.err
		case <-ctx.Done():
			return domain.TVEpisodeMetadata{}, ctx.Err()
		}
	}
	call := &episodeCall{done: make(chan struct{})}
	client.episodeCalls[cacheKey] = call
	client.mu.Unlock()
	episode, err := client.fetchEpisode(ctx, showID, seasonNumber, episodeNumber)
	client.mu.Lock()
	call.episode = cloneEpisode(episode)
	call.err = err
	if err == nil {
		client.episodeCache[cacheKey] = cachedEpisode{episode: cloneEpisode(episode), expiresAt: time.Now().Add(showCacheTTL)}
	}
	delete(client.episodeCalls, cacheKey)
	close(call.done)
	client.mu.Unlock()
	return cloneEpisode(episode), err
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

func (client *Client) fetchShow(ctx context.Context, tmdbID int64) (domain.TVShowMetadata, error) {
	show, err := client.fetchShowSummary(ctx, tmdbID)
	if err != nil {
		return domain.TVShowMetadata{}, err
	}
	for index := range show.Seasons {
		season, err := client.fetchSeason(ctx, tmdbID, show.Seasons[index].Number)
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
