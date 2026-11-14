package omdb

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRatingsParsesOnlyValidMatchingScores(t *testing.T) {
	for _, test := range []struct {
		name, body   string
		imdb, rotten bool
		wantError    bool
	}{
		{"both scores", `{"Response":"True","imdbID":"tt0111161","Ratings":[{"Source":"Internet Movie Database","Value":"9.3/10"},{"Source":"Rotten Tomatoes","Value":"91%"}]}`, true, true, false},
		{"missing scores", `{"Response":"True","imdbID":"tt0111161","Ratings":[{"Source":"Internet Movie Database","Value":"N/A"}]}`, false, false, false},
		{"invalid ranges", `{"Response":"True","imdbID":"tt0111161","Ratings":[{"Source":"Internet Movie Database","Value":"NaN/10"},{"Source":"Rotten Tomatoes","Value":"101%"}]}`, false, false, false},
		{"zero is real", `{"Response":"True","imdbID":"tt0111161","Ratings":[{"Source":"Rotten Tomatoes","Value":"0%"}]}`, false, true, false},
		{"wrong title", `{"Response":"True","imdbID":"tt1234567"}`, false, false, true},
		{"quota", `{"Response":"False","Error":"Request limit reached!"}`, false, false, true},
		{"not found", `{"Response":"False","Error":"Movie not found!"}`, false, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := New("private-key", &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Scheme != "https" || request.URL.Query().Get("i") != "tt0111161" || request.URL.Query().Get("apikey") != "private-key" {
					t.Fatal("incorrect provider request")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})})
			scores, err := client.Ratings(context.Background(), "tt0111161")
			if (err != nil) != test.wantError || (scores.IMDb != nil) != test.imdb || (scores.RottenTomatoes != nil) != test.rotten {
				t.Fatalf("scores=%+v err=%v", scores, err)
			}
		})
	}
}

func TestRatingsNeverExposesAPIKeyOnTransportFailure(t *testing.T) {
	client := New("private-key", &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("failure") })})
	_, err := client.Ratings(context.Background(), "tt0111161")
	if err == nil || strings.Contains(err.Error(), "private-key") {
		t.Fatalf("unsafe error: %v", err)
	}
}
