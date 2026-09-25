package tracking_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/tracking"
)

type repository struct {
	plays []tracking.Play
}

func (repository *repository) CreatePlay(_ context.Context, play tracking.Play) (tracking.Play, error) {
	play.ID = "1"
	repository.plays = append(repository.plays, play)
	return play, nil
}
func (repository *repository) CreateBulkPlays(_ context.Context, plays []tracking.Play) ([]tracking.Play, error) {
	for index := range plays {
		plays[index].ID = fmt.Sprint(index + 1)
	}
	repository.plays = append(repository.plays, plays...)
	return plays, nil
}
func (repository *repository) UpdatePlay(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (repository *repository) DeletePlay(_ context.Context, _, _ string) error { return nil }

func TestRecord_GivenFutureTimestamp_WhenRecordingLeakEpisode_ThenItRejectsTheTimestamp(t *testing.T) {
	repository := &repository{}
	service := tracking.NewService(repository)
	serviceNow := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	// The service's wall clock is intentionally not injectable outside its package;
	// this test uses a timestamp far enough in the future for every execution.
	episodeID := "episode-1"
	_, err := service.Record(context.Background(), "user-1", nil, &episodeID, serviceNow.AddDate(100, 0, 0), "web")
	if !errors.Is(err, tracking.ErrFutureWatchTime) {
		t.Fatalf("error = %v, want future timestamp error", err)
	}
	if len(repository.plays) != 0 {
		t.Fatal("future play must not be stored")
	}
}

func TestRecord_GivenEpisodeAndCurrentTime_WhenRecording_ThenItStoresAnIndividualPlay(t *testing.T) {
	repository := &repository{}
	service := tracking.NewService(repository)
	episodeID := "episode-1"
	play, err := service.Record(context.Background(), "user-1", nil, &episodeID, time.Time{}, "web")
	if err != nil {
		t.Fatalf("record play: %v", err)
	}
	if play.ID != "1" || len(repository.plays) != 1 || repository.plays[0].EpisodeID == nil {
		t.Fatalf("play = %#v", play)
	}
}

func TestRecordEpisodes_GivenSkippedEpisodes_WhenRecordingBulk_ThenItStoresEachAsAnIndividualPlay(t *testing.T) {
	repo := &repository{}
	service := tracking.NewService(repo)
	plays, err := service.RecordEpisodes(context.Background(), "user-1", []string{"episode-1", "episode-2"}, time.Time{}, "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(plays) != 2 || len(repo.plays) != 2 || repo.plays[0].EpisodeID == nil || repo.plays[1].EpisodeID == nil {
		t.Fatalf("plays=%+v stored=%+v", plays, repo.plays)
	}
}

func TestRemoveMediaPlays_GivenATVShowID_WhenRemovingByMedia_ThenItRejectsTheRequest(t *testing.T) {
	service := tracking.NewService(&repository{})
	err := service.RemoveMediaPlays(context.Background(), "user-1", "tv:42")
	if err == nil {
		t.Fatal("expected non-movie media ID to be rejected")
	}
}

func TestCorrect_GivenFutureTimestamp_WhenCorrecting_ThenItRejectsTheTimestamp(t *testing.T) {
	service := tracking.NewService(&repository{})
	err := service.Correct(context.Background(), "user-1", "play-1", time.Now().UTC().AddDate(100, 0, 0))
	if !errors.Is(err, tracking.ErrFutureWatchTime) {
		t.Fatalf("error = %v, want future timestamp error", err)
	}
}
