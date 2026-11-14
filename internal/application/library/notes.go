package library

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"
)

const MaxNoteLength = 5000

var ErrInvalidNote = errors.New("invalid note")

type noteRepository interface {
	GetNote(context.Context, string, string) (string, error)
	SetNote(context.Context, string, string, string) error
}

func (s *Service) Note(ctx context.Context, userID, mediaID string) (string, error) {
	repository, ok := s.repository.(noteRepository)
	if !ok {
		return "", fmt.Errorf("notes are not configured")
	}
	return repository.GetNote(ctx, userID, mediaID)
}

func (s *Service) SetNote(ctx context.Context, userID, mediaID, note string) error {
	if err := validateNote(note); err != nil {
		return err
	}
	repository, ok := s.repository.(noteRepository)
	if !ok {
		return fmt.Errorf("notes are not configured")
	}
	return repository.SetNote(ctx, userID, mediaID, note)
}

func validateNote(note string) error {
	if !utf8.ValidString(note) || utf8.RuneCountInString(note) > MaxNoteLength {
		return fmt.Errorf("%w: use valid text of at most %d characters", ErrInvalidNote, MaxNoteLength)
	}
	return nil
}

type episodeNoteRepository interface {
	GetEpisodeNote(context.Context, string, string) (string, error)
	SetEpisodeNote(context.Context, string, string, string) error
}

func (s *Service) EpisodeNote(ctx context.Context, userID, episodeID string) (string, error) {
	repository, ok := s.repository.(episodeNoteRepository)
	if !ok {
		return "", fmt.Errorf("episode notes are not configured")
	}
	return repository.GetEpisodeNote(ctx, userID, episodeID)
}

func (s *Service) SetEpisodeNote(ctx context.Context, userID, episodeID, note string) error {
	if err := validateNote(note); err != nil {
		return err
	}
	repository, ok := s.repository.(episodeNoteRepository)
	if !ok {
		return fmt.Errorf("episode notes are not configured")
	}
	return repository.SetEpisodeNote(ctx, userID, episodeID, note)
}
