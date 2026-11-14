package tmdb

import (
	"context"
	"fmt"

	"github.com/afonsocosta/visto/internal/domain"
)

func (client *Client) IMDbID(ctx context.Context, mediaType domain.MediaType, tmdbID int64) (string, error) {
	if tmdbID <= 0 || (mediaType != domain.MovieMediaType && mediaType != domain.TVMediaType) {
		return "", fmt.Errorf("valid media type and TMDB ID are required")
	}
	var id string
	err := client.request(ctx, func() error {
		if mediaType == domain.MovieMediaType {
			external, err := client.client.GetMovieExternalIDs(int(tmdbID), nil)
			if err != nil {
				return err
			}
			id = external.IMDbID
		} else {
			external, err := client.client.GetTVExternalIDs(int(tmdbID), nil)
			if err != nil {
				return err
			}
			id = external.IMDbID
		}
		return nil
	})
	return id, err
}

func (client *Client) EpisodeIMDbID(ctx context.Context, showID int64, season, episode int) (string, error) {
	if showID <= 0 || season < 0 || episode <= 0 {
		return "", fmt.Errorf("valid show, season and episode are required")
	}
	var id string
	err := client.request(ctx, func() error {
		external, err := client.client.GetTVEpisodeExternalIDs(int(showID), season, episode)
		if err != nil {
			return err
		}
		id = external.IMDbID
		return nil
	})
	return id, err
}
