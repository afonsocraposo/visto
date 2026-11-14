package omdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/application/ratings"
)

var imdbIDPattern = regexp.MustCompile(`^tt[0-9]{7,}$`)

type Client struct {
	key        string
	httpClient *http.Client
}

func New(key string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &Client{key: strings.TrimSpace(key), httpClient: client}
}

func (c *Client) Ratings(ctx context.Context, imdbID string) (ratings.Scores, error) {
	var scores ratings.Scores
	if !imdbIDPattern.MatchString(imdbID) {
		return scores, fmt.Errorf("invalid IMDb ID")
	}
	query := url.Values{"apikey": {c.key}, "i": {imdbID}, "r": {"json"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.omdbapi.com/?"+query.Encode(), nil)
	if err != nil {
		return scores, fmt.Errorf("could not create ratings request")
	}
	response, err := c.httpClient.Do(request)
	// Do not return transport errors: their URLs can contain the API key.
	if err != nil {
		return scores, fmt.Errorf("ratings provider is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return scores, fmt.Errorf("ratings provider returned status %d", response.StatusCode)
	}
	var body struct {
		Response   string
		Error      string
		IMDbID     string `json:"imdbID"`
		IMDbRating string `json:"imdbRating"`
		Ratings    []struct{ Source, Value string }
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return scores, fmt.Errorf("invalid ratings response")
	}
	if body.Response == "False" {
		if body.Error == "Movie not found!" {
			return ratings.Scores{IMDbID: imdbID}, nil
		}
		return scores, fmt.Errorf("ratings provider could not complete the request")
	}
	if body.Response != "True" || body.IMDbID != imdbID {
		return scores, fmt.Errorf("ratings response does not match the title")
	}
	scores.IMDbID = imdbID
	// Episode responses can provide imdbRating without an entry in Ratings.
	if value, err := strconv.ParseFloat(body.IMDbRating, 64); err == nil && !math.IsNaN(value) && value >= 0 && value <= 10 {
		scores.IMDb = &value
	}
	for _, rating := range body.Ratings {
		switch rating.Source {
		case "Internet Movie Database":
			if !strings.HasSuffix(rating.Value, "/10") {
				continue
			}
			value, err := strconv.ParseFloat(strings.TrimSuffix(rating.Value, "/10"), 64)
			if err == nil && !math.IsNaN(value) && value >= 0 && value <= 10 {
				scores.IMDb = &value
			}
		case "Rotten Tomatoes":
			if !strings.HasSuffix(rating.Value, "%") {
				continue
			}
			value, err := strconv.Atoi(strings.TrimSuffix(rating.Value, "%"))
			if err == nil && value >= 0 && value <= 100 {
				scores.RottenTomatoes = &value
			}
		}
	}
	return scores, nil
}
