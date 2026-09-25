package httpserver

import "encoding/csv"
import "net/http"
import "strconv"
import "time"
import "github.com/afonsocosta/visto/internal/application/auth"
import exportapp "github.com/afonsocosta/visto/internal/application/export"

func jsonExport(authService *auth.Service, service *exportapp.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "export is not configured")
			return
		}
		data, err := service.Data(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export is temporarily unavailable")
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=visto-export.json")
		writeJSON(w, http.StatusOK, data)
	}
}

func csvExport(authService *auth.Service, service *exportapp.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if service == nil {
			writeError(w, http.StatusServiceUnavailable, "export is not configured")
			return
		}
		data, err := service.Data(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export is temporarily unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=visto-export.csv")
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"kind", "id", "media_id", "episode_id", "title", "type", "status", "rating", "watched_at"})
		for _, item := range data.Library {
			rating := ""
			if item.Rating != nil {
				rating = strconv.Itoa(*item.Rating)
			}
			_ = writer.Write([]string{"library", "", item.MediaID, "", item.Title, item.Type, item.Status, rating, ""})
		}
		for _, play := range data.Plays {
			mediaID, episodeID := "", ""
			if play.MediaID != nil {
				mediaID = *play.MediaID
			}
			if play.EpisodeID != nil {
				episodeID = *play.EpisodeID
			}
			_ = writer.Write([]string{"play", play.ID, mediaID, episodeID, "", "", "", "", play.WatchedAt.Format(time.RFC3339Nano)})
		}
		for _, rating := range data.EpisodeRatings {
			value := ""
			if rating.Rating != nil {
				value = strconv.Itoa(*rating.Rating)
			}
			_ = writer.Write([]string{"episode_rating", "", "", rating.EpisodeID, "", "episode", "", value, rating.UpdatedAt.Format(time.RFC3339Nano)})
		}
		writer.Flush()
	}
}
