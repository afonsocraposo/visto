package importer

import (
	"context"
	"errors"
	"fmt"
)

var ErrInvalidArchive = errors.New("invalid import archive")

type Repository interface {
	ImportData(context.Context, string, Data) (Result, error)
	ImportWelcomePending(context.Context, string) (bool, error)
	DismissImportWelcome(context.Context, string) error
}

type TVDBResolver interface {
	FindByTVDB(context.Context, string, string) (int64, error)
}

type Service struct {
	repository Repository
	resolver   TVDBResolver
	wake       func()
}

func NewService(repository Repository, resolver TVDBResolver, wake ...func()) *Service {
	service := &Service{repository: repository, resolver: resolver}
	if len(wake) > 0 {
		service.wake = wake[0]
	}
	return service
}

func (s *Service) Import(ctx context.Context, userID, source string, payload []byte) (Result, error) {
	if userID == "" {
		return Result{}, fmt.Errorf("user is required")
	}
	d, err := Parse(source, payload)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	resolved := map[string]int64{}
	resolve := func(kind, tvdb string, id *int64) {
		if *id > 0 {
			return
		}
		key := kind + ":" + tvdb
		if known, ok := resolved[key]; ok {
			*id = known
			return
		}
		if tvdb != "" && s.resolver != nil {
			found, err := s.resolver.FindByTVDB(ctx, kind, tvdb)
			if err == nil && found > 0 {
				resolved[key] = found
				*id = found
				return
			}
		}
		resolved[key] = 0
	}
	for i := range d.Titles {
		resolve(d.Titles[i].Type, d.Titles[i].TVDBID, &d.Titles[i].TMDBID)
	}
	for i := range d.Watches {
		kind := d.Watches[i].Type
		if kind == "episode" {
			kind = "tv"
		}
		resolve(kind, d.Watches[i].TVDBID, &d.Watches[i].TMDBID)
	}
	for i := range d.Ratings {
		kind := d.Ratings[i].Type
		if kind == "episode" {
			kind = "tv"
		}
		resolve(kind, d.Ratings[i].TVDBID, &d.Ratings[i].TMDBID)
	}
	result, err := s.repository.ImportData(ctx, userID, d)
	if err == nil && s.wake != nil {
		s.wake()
	}
	return result, err
}

func (s *Service) WelcomePending(ctx context.Context, userID string) (bool, error) {
	return s.repository.ImportWelcomePending(ctx, userID)
}
func (s *Service) DismissWelcome(ctx context.Context, userID string) error {
	return s.repository.DismissImportWelcome(ctx, userID)
}
