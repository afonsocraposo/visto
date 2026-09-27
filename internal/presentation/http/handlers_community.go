package httpserver

import (
	"errors"
	"net/http"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/application/feed"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"github.com/afonsocosta/visto/internal/application/profile"
)

type communityUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type communityProfile struct {
	communityUser
	Sharing bool `json:"sharing"`
}

type communityLibraryItem struct {
	Media    library.Media         `json:"media"`
	Status   string                `json:"status"`
	Rating   *int                  `json:"rating"`
	Progress *library.ShowProgress `json:"progress,omitempty"`
}

func communityUsers(authService *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedUser(w, r, authService); !ok {
			return
		}
		request, err := pagination.Parse(r)
		if err != nil {
			writePageError(w, err, "could not list users")
			return
		}
		page, err := authService.UsersPage(r.Context(), request)
		if err != nil {
			writePageError(w, err, "could not list users")
			return
		}
		items := make([]communityUser, 0, len(page.Items))
		for _, user := range page.Items {
			items = append(items, communityUser{ID: user.ID, Name: user.DisplayName})
		}
		writeJSON(w, http.StatusOK, pagination.Page[communityUser]{Items: items, NextCursor: page.NextCursor})
	}
}

func sharedUser(w http.ResponseWriter, r *http.Request, authService *auth.Service, profileService *profile.Service) (communityProfile, bool) {
	if _, ok := authenticatedUser(w, r, authService); !ok {
		return communityProfile{}, false
	}
	user, err := authService.CommunityUser(r.Context(), r.PathValue("userID"))
	if errors.Is(err, auth.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return communityProfile{}, false
	}
	if err != nil || profileService == nil {
		writeError(w, http.StatusServiceUnavailable, "profile is temporarily unavailable")
		return communityProfile{}, false
	}
	settings, err := profileService.Get(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "profile is temporarily unavailable")
		return communityProfile{}, false
	}
	return communityProfile{communityUser: communityUser{ID: user.ID, Name: user.DisplayName}, Sharing: settings.ActivityVisibility == profile.InstanceVisibility}, true
}

func communityUserProfile(authService *auth.Service, profileService *profile.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		person, ok := sharedUser(w, r, authService, profileService)
		if ok {
			writeJSON(w, http.StatusOK, person)
		}
	}
}

func communityUserLibrary(authService *auth.Service, profileService *profile.Service, service *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		person, ok := sharedUser(w, r, authService, profileService)
		if !ok {
			return
		}
		if !person.Sharing {
			writeError(w, http.StatusForbidden, "this library is private")
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "library is not configured")
			return
		}
		request, err := pagination.Parse(r)
		if err != nil {
			writePageError(w, err, "library is temporarily unavailable")
			return
		}
		page, err := service.ListPage(r.Context(), person.ID, library.ListOptions{Sort: r.URL.Query().Get("sort"), Status: r.URL.Query().Get("status"), MediaType: r.URL.Query().Get("media_type")}, request)
		if err != nil {
			writePageError(w, err, "library is temporarily unavailable")
			return
		}
		items := make([]communityLibraryItem, 0, len(page.Items))
		for _, entry := range page.Items {
			items = append(items, communityLibraryItem{Media: entry.Media, Status: string(entry.Item.Status), Rating: entry.Item.Rating, Progress: entry.Progress})
		}
		writeJSON(w, http.StatusOK, pagination.Page[communityLibraryItem]{Items: items, NextCursor: page.NextCursor, TotalCount: page.TotalCount})
	}
}

func communityUserActivity(authService *auth.Service, profileService *profile.Service, service *feed.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		person, ok := sharedUser(w, r, authService, profileService)
		if !ok {
			return
		}
		if !person.Sharing {
			writeError(w, http.StatusForbidden, "this activity is private")
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "feed is not configured")
			return
		}
		request, err := pagination.Parse(r)
		if err != nil {
			writePageError(w, err, "activity is temporarily unavailable")
			return
		}
		page, err := service.ListUser(r.Context(), person.ID, request.Cursor, request.Limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}
