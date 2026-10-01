package tmdb

import (
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

func cloneSlice[T any](items []T) []T {
	if items == nil {
		return nil
	}
	cloned := make([]T, len(items))
	copy(cloned, items)
	return cloned
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
	clone.Seasons = cloneSlice(show.Seasons)
	for index, season := range show.Seasons {
		clone.Seasons[index] = season
		clone.Seasons[index].Episodes = cloneSlice(season.Episodes)
	}
	clone.Cast = cloneSlice(show.Cast)
	return clone
}

func cloneSeason(season domain.TVSeasonMetadata) domain.TVSeasonMetadata {
	clone := season
	clone.Episodes = cloneSlice(season.Episodes)
	return clone
}

func cloneEpisode(episode domain.TVEpisodeMetadata) domain.TVEpisodeMetadata {
	clone := episode
	clone.GuestStars = cloneSlice(episode.GuestStars)
	clone.Crew = cloneSlice(episode.Crew)
	return clone
}

func cloneMovie(movie domain.MovieMetadata) domain.MovieMetadata {
	clone := movie
	clone.Genres = cloneSlice(movie.Genres)
	clone.Cast = cloneSlice(movie.Cast)
	return clone
}

func clonePerson(person domain.PersonMetadata) domain.PersonMetadata {
	clone := person
	clone.Credits = cloneSlice(person.Credits)
	return clone
}
