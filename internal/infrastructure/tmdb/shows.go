package tmdb

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	tmdbapi "github.com/cyruzin/golang-tmdb"
)

// maxAggregateCharacters caps how many of an actor's series roles are shown.
const maxAggregateCharacters = 2

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
		details, requestErr = client.client.GetTVDetails(int(tmdbID), map[string]string{"append_to_response": "aggregate_credits"})
		return requestErr
	}); err != nil {
		return domain.TVShowMetadata{}, err
	}
	show := domain.TVShowMetadata{TMDBID: details.ID, Name: details.Name, Overview: details.Overview, PosterPath: details.PosterPath, BackdropPath: details.BackdropPath, FirstAirDate: details.FirstAirDate, OriginalLanguage: details.OriginalLanguage, Status: details.Status}
	if details.TVAggregateCreditsAppend != nil && details.AggregateCredits != nil {
		show.Cast = aggregateCast(details.AggregateCredits)
	}
	for _, season := range details.Seasons {
		show.Seasons = append(show.Seasons, domain.TVSeasonMetadata{TMDBID: season.ID, Number: season.SeasonNumber, EpisodeCount: season.EpisodeCount, Name: season.Name, Overview: season.Overview, PosterPath: season.PosterPath, AirDate: season.AirDate})
	}
	return show, nil
}

// aggregateCast maps series-wide credits so long-running shows list their
// principal ensemble rather than only the latest season's cast.
func aggregateCast(credits *tmdbapi.TVAggregateCredits) []domain.TVCastMember {
	type rankedMember struct {
		member   domain.TVCastMember
		order    int
		episodes int
	}
	ranked := make([]rankedMember, 0, len(credits.Cast))
	for _, actor := range credits.Cast {
		// Roles with the most episodes describe the actor's main character.
		roles := make([]int, len(actor.Roles))
		for index := range roles {
			roles[index] = index
		}
		slices.SortStableFunc(roles, func(a, b int) int {
			return actor.Roles[b].EpisodeCount - actor.Roles[a].EpisodeCount
		})
		characters := []string{}
		for _, index := range roles {
			character := strings.TrimSpace(actor.Roles[index].Character)
			if character != "" && !slices.Contains(characters, character) && len(characters) < maxAggregateCharacters {
				characters = append(characters, character)
			}
		}
		ranked = append(ranked, rankedMember{
			member:   domain.TVCastMember{ID: actor.ID, Name: actor.Name, Character: strings.Join(characters, " / "), ProfilePath: actor.ProfilePath},
			order:    actor.Order,
			episodes: actor.TotalEpisodeCount,
		})
	}
	slices.SortStableFunc(ranked, func(a, b rankedMember) int {
		if a.order != b.order {
			return a.order - b.order
		}
		return b.episodes - a.episodes
	})
	cast := make([]domain.TVCastMember, 0, len(ranked))
	for _, item := range ranked {
		cast = append(cast, item.member)
	}
	return cast
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
	season := domain.TVSeasonMetadata{TMDBID: seasonDetails.ID, Number: seasonDetails.SeasonNumber, Name: seasonDetails.Name, Overview: seasonDetails.Overview, PosterPath: seasonDetails.PosterPath, AirDate: seasonDetails.AirDate, Episodes: []domain.TVEpisodeMetadata{}}
	season.EpisodeCount = len(seasonDetails.Episodes)
	for _, episode := range seasonDetails.Episodes {
		season.Episodes = append(season.Episodes, domain.TVEpisodeMetadata{TMDBID: episode.ID, SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber, Name: episode.Name, Overview: episode.Overview, AirDate: episode.AirDate, Runtime: episode.Runtime, StillPath: episode.StillPath})
	}
	return season, nil
}
