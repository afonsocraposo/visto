package tmdb

import (
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

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
