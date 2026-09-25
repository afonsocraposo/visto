package tmdb

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
	tmdbapi "github.com/cyruzin/golang-tmdb"
)

func (client *Client) Movie(ctx context.Context, tmdbID int64) (domain.MovieMetadata, error) {
	if tmdbID <= 0 {
		return domain.MovieMetadata{}, fmt.Errorf("TMDB movie ID must be positive")
	}
	client.mu.Lock()
	if cached, ok := client.movieCache[tmdbID]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return cloneMovie(cached.movie), nil
	}
	client.mu.Unlock()
	result, err := client.coalesce(ctx, fmt.Sprintf("movie:%d", tmdbID), func() (any, error) {
		movie, err := client.fetchMovie(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.movieCache[tmdbID] = cachedMovie{movie: cloneMovie(movie), expiresAt: time.Now().Add(showCacheTTL)}
		client.mu.Unlock()
		return movie, nil
	})
	if err != nil {
		return domain.MovieMetadata{}, err
	}
	return cloneMovie(result.(domain.MovieMetadata)), nil
}

func (client *Client) Person(ctx context.Context, tmdbID int64) (domain.PersonMetadata, error) {
	if tmdbID <= 0 {
		return domain.PersonMetadata{}, fmt.Errorf("TMDB person ID must be positive")
	}
	client.mu.Lock()
	if cached, ok := client.personCache[tmdbID]; ok && time.Now().Before(cached.expiresAt) {
		client.mu.Unlock()
		return clonePerson(cached.person), nil
	}
	client.mu.Unlock()
	result, err := client.coalesce(ctx, fmt.Sprintf("person:%d", tmdbID), func() (any, error) {
		person, err := client.fetchPerson(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.personCache[tmdbID] = cachedPerson{person: clonePerson(person), expiresAt: time.Now().Add(showCacheTTL)}
		now := time.Now()
		for id, cached := range client.personCache {
			if !now.Before(cached.expiresAt) {
				delete(client.personCache, id)
			}
		}
		for len(client.personCache) > maxShowCacheEntries {
			for id := range client.personCache {
				if id != tmdbID {
					delete(client.personCache, id)
					break
				}
			}
		}
		client.mu.Unlock()
		return person, nil
	})
	if err != nil {
		return domain.PersonMetadata{}, err
	}
	return clonePerson(result.(domain.PersonMetadata)), nil
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
	client.mu.Unlock()
	result, err := client.coalesce(ctx, "episode:"+cacheKey, func() (any, error) {
		episode, err := client.fetchEpisode(ctx, showID, seasonNumber, episodeNumber)
		if err != nil {
			return nil, err
		}
		client.mu.Lock()
		client.episodeCache[cacheKey] = cachedEpisode{episode: cloneEpisode(episode), expiresAt: time.Now().Add(showCacheTTL)}
		client.mu.Unlock()
		return episode, nil
	})
	if err != nil {
		return domain.TVEpisodeMetadata{}, err
	}
	return cloneEpisode(result.(domain.TVEpisodeMetadata)), nil
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

func (client *Client) fetchPerson(ctx context.Context, tmdbID int64) (domain.PersonMetadata, error) {
	if err := client.acquire(ctx); err != nil {
		return domain.PersonMetadata{}, err
	}
	defer func() { <-client.requests }()
	var details *tmdbapi.PersonDetails
	if err := client.requestHeld(ctx, func() error {
		var requestErr error
		details, requestErr = client.client.GetPersonDetails(int(tmdbID), map[string]string{"append_to_response": "combined_credits"})
		return requestErr
	}); err != nil {
		return domain.PersonMetadata{}, err
	}
	person := domain.PersonMetadata{TMDBID: details.ID, Name: details.Name, Biography: details.Biography, ProfilePath: details.ProfilePath, Birthday: details.Birthday, Deathday: details.Deathday, PlaceOfBirth: details.PlaceOfBirth, KnownFor: details.KnownForDepartment, Credits: []domain.PersonCredit{}}
	seen := make(map[string]int)
	if details.PersonCombinedCreditsAppend != nil && details.CombinedCredits != nil {
		for _, credit := range details.CombinedCredits.Cast {
			mediaType := domain.MediaType(credit.MediaType)
			if (mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType) || credit.Adult || credit.ID <= 0 {
				continue
			}
			title, originalTitle, releaseDate := credit.Title, credit.OriginalTitle, credit.ReleaseDate
			if mediaType == domain.TVMediaType {
				title, originalTitle, releaseDate = credit.Name, credit.OriginalName, credit.FirstAirDate
			}
			if title == "" {
				continue
			}
			item := domain.PersonCredit{TMDBID: credit.ID, Type: mediaType, Title: title, OriginalTitle: originalTitle, Character: credit.Character, Overview: credit.Overview, ReleaseDate: releaseDate, PosterPath: credit.PosterPath, BackdropPath: credit.BackdropPath, OriginalLanguage: credit.OriginalLanguage, Popularity: credit.Popularity}
			key := fmt.Sprintf("%s:%d", mediaType, credit.ID)
			if index, exists := seen[key]; exists {
				if person.Credits[index].Character == "" && item.Character != "" {
					person.Credits[index].Character = item.Character
				}
				continue
			}
			seen[key] = len(person.Credits)
			person.Credits = append(person.Credits, item)
		}
	}
	sort.SliceStable(person.Credits, func(i, j int) bool {
		if person.Credits[i].Popularity != person.Credits[j].Popularity {
			return person.Credits[i].Popularity > person.Credits[j].Popularity
		}
		return person.Credits[i].Title < person.Credits[j].Title
	})
	return person, nil
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
