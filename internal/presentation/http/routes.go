package httpserver

import (
	"net/http"

	"github.com/afonsocosta/visto/internal/application/auth"
	exportapp "github.com/afonsocosta/visto/internal/application/export"
	"github.com/afonsocosta/visto/internal/application/feed"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/profile"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/presentation/security"
)

// registerAPIRoutes keeps the HTTP surface in one small, auditable place.
// Handler implementations stay grouped by domain in their respective files.
func registerAPIRoutes(mux *http.ServeMux, authService *auth.Service, metadataProvider domain.MetadataProvider, libraryService *library.Service, trackingService *tracking.Service, profileService *profile.Service, feedService *feed.Service, exportService *exportapp.Service, watchService *watch.Service, loginLimiter *LoginLimiter, proxies *security.ProxyResolver) {
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", bootstrap(authService))
	mux.HandleFunc("GET /api/v1/auth/status", bootstrapStatus(authService))
	mux.HandleFunc("POST /api/v1/auth/login", login(authService, loginLimiter, proxies))
	mux.HandleFunc("POST /api/v1/auth/signup", signup(authService, proxies))
	mux.HandleFunc("POST /api/v1/auth/logout", logout(authService, proxies))
	mux.HandleFunc("POST /api/v1/users", createUser(authService))
	mux.HandleFunc("GET /api/v1/users", listUsers(authService))
	mux.HandleFunc("PATCH /api/v1/users/{userID}", updateUser(authService))
	mux.HandleFunc("DELETE /api/v1/users/{userID}", deleteUser(authService))
	mux.HandleFunc("GET /api/v1/me", currentUser(authService))
	mux.HandleFunc("GET /api/v1/tokens", listPersonalTokens(authService))
	mux.HandleFunc("POST /api/v1/tokens", createPersonalToken(authService))
	mux.HandleFunc("DELETE /api/v1/tokens/{tokenID}", revokePersonalToken(authService))
	mux.HandleFunc("GET /api/v1/profile/activity-settings", activitySettings(authService, profileService))
	mux.HandleFunc("PATCH /api/v1/profile/activity-settings", setActivitySettings(authService, profileService))
	mux.HandleFunc("PATCH /api/v1/profile/pushover-settings", setPushoverSettings(authService, profileService))
	mux.HandleFunc("DELETE /api/v1/profile/pushover-credentials", clearPushoverCredentials(authService, profileService))
	mux.HandleFunc("GET /api/v1/feed", instanceFeed(authService, feedService))
	mux.HandleFunc("GET /api/v1/export/json", jsonExport(authService, exportService))
	mux.HandleFunc("GET /api/v1/export/csv", csvExport(authService, exportService))
	mux.HandleFunc("GET /api/v1/search", search(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/trending", trending(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/public/trending", publicTrending(metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/shows/{tmdbID}", temporaryShowDetails(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/{mediaType}/{tmdbID}/related", relatedMedia(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/shows/{tmdbID}/seasons/{seasonNumber}", temporaryShowSeasonEpisodes(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/shows/{tmdbID}/seasons/{seasonNumber}/episodes/{episodeNumber}", temporaryEpisodeDetails(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/discover/movies/{tmdbID}", temporaryMovieDetails(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/people/{tmdbID}", personDetails(authService, metadataProvider))
	mux.HandleFunc("GET /api/v1/movies/{tmdbID}", mediaDetails(authService, libraryService, metadataProvider, domain.MovieMediaType))
	mux.HandleFunc("GET /api/v1/shows/{tmdbID}", mediaDetails(authService, libraryService, metadataProvider, domain.TVMediaType))
	mux.HandleFunc("GET /api/v1/library", listLibrary(authService, libraryService))
	mux.HandleFunc("POST /api/v1/library", saveLibrary(authService, libraryService, metadataProvider))
	mux.HandleFunc("PATCH /api/v1/library/{mediaID}", updateLibrary(authService, libraryService))
	mux.HandleFunc("DELETE /api/v1/library/{mediaID}", removeWatchlistItem(authService, libraryService))
	mux.HandleFunc("PATCH /api/v1/library/{mediaID}/notifications", setLibraryNotifications(authService, libraryService))
	mux.HandleFunc("POST /api/v1/plays", createPlay(authService, trackingService))
	mux.HandleFunc("GET /api/v1/plays", playHistory(authService, trackingService))
	mux.HandleFunc("POST /api/v1/plays/bulk", createBulkPlays(authService, trackingService))
	mux.HandleFunc("DELETE /api/v1/plays/bulk", deleteBulkEpisodePlays(authService, trackingService))
	mux.HandleFunc("DELETE /api/v1/plays/media/{mediaID}", deleteMediaPlays(authService, trackingService))
	mux.HandleFunc("PATCH /api/v1/plays/{playID}", correctPlay(authService, trackingService))
	mux.HandleFunc("DELETE /api/v1/plays/{playID}", deletePlay(authService, trackingService))
	mux.HandleFunc("GET /api/v1/episodes/{episodeID}/rating", episodeRating(authService, trackingService))
	mux.HandleFunc("PUT /api/v1/episodes/{episodeID}/rating", setEpisodeRating(authService, trackingService))
	mux.HandleFunc("GET /api/v1/continue-watching", continueWatching(authService, watchService))
	mux.HandleFunc("GET /api/v1/calendar", calendar(authService, watchService))
	mux.HandleFunc("GET /api/v1/shows/{showID}/progress", showProgress(authService, watchService))
	mux.HandleFunc("GET /api/v1/shows/{showID}/seasons", showSeasons(authService, watchService))
	mux.HandleFunc("GET /api/v1/shows/{showID}/episodes", showEpisodes(authService, watchService))
	mux.HandleFunc("POST /api/v1/shows/{showID}/episodes/watch-through", watchThroughEpisodes(authService, trackingService, watchService))
	mux.HandleFunc("GET /api/v1/seasons/{seasonID}/episodes", seasonEpisodes(authService, watchService))
}
