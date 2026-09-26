package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/backup"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func adminBackup(authService *auth.Service, service *backup.Service, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if user.Role != domain.AdminRole {
			writeError(w, 403, "administrator access required")
			return
		}
		ctx := r.Context()
		switch action {
		case "get":
			b, err := service.Settings(ctx)
			if err != nil {
				writeError(w, 500, "could not load backup settings")
				return
			}
			writeJSON(w, 200, b)
		case "save":
			var request struct {
				sqlite.BackupSettings
				SecretKey string `json:"secret_key"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				writeError(w, 400, "invalid JSON")
				return
			}
			if err := service.Save(ctx, request.BackupSettings, request.SecretKey); err != nil {
				writeError(w, 400, err.Error())
				return
			}
			b, _ := service.Settings(ctx)
			writeJSON(w, 200, b)
		case "test":
			testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := service.Test(testCtx); err != nil {
				writeError(w, 400, "S3 connection test failed")
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
		case "run":
			go func() { _ = service.RunOnce(context.Background(), false) }()
			writeJSON(w, 202, map[string]bool{"started": true})
		}
	}
}
