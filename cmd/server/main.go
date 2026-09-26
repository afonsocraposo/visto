package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/afonsocosta/visto/internal/application/activity"
	"github.com/afonsocosta/visto/internal/application/auth"
	exportapp "github.com/afonsocosta/visto/internal/application/export"
	"github.com/afonsocosta/visto/internal/application/feed"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/notifications"
	"github.com/afonsocosta/visto/internal/application/oauth"
	"github.com/afonsocosta/visto/internal/application/plexsync"
	"github.com/afonsocosta/visto/internal/application/profile"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	backupjob "github.com/afonsocosta/visto/internal/infrastructure/backup"
	"github.com/afonsocosta/visto/internal/infrastructure/pushover"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	"github.com/afonsocosta/visto/internal/infrastructure/tmdb"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
	mcpserver "github.com/afonsocosta/visto/internal/presentation/mcp"
	"github.com/afonsocosta/visto/internal/presentation/security"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "backup" {
		if len(os.Args) != 3 {
			log.Fatal("usage: visto backup <destination.db>")
		}
		if err := runBackup(environment("VISTO_DATABASE_PATH", "./data/visto.db"), os.Args[2]); err != nil {
			log.Fatalf("create backup: %v", err)
		}
		log.Printf("database backup created at %s", os.Args[2])
		return
	}
	databasePath := environment("VISTO_DATABASE_PATH", "./data/visto.db")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
		log.Fatalf("create data directory: %v", err)
	}
	store, err := sqlite.Open(context.Background(), databasePath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()
	backgroundContext, stopBackground := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	startWorker := func(run func(context.Context)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			run(backgroundContext)
		}()
	}

	var metadataProvider *tmdb.Client
	if apiKey := os.Getenv("VISTO_TMDB_API_KEY"); apiKey != "" {
		metadataProvider, err = tmdb.New(apiKey, nil)
		if err != nil {
			log.Fatalf("configure TMDB: %v", err)
		}
	}
	watchService := watch.NewService(store, metadataProvider)
	refreshInterval := durationEnvironment("VISTO_CATALOG_REFRESH_INTERVAL", 6*time.Hour)
	activeRefreshTTL := durationEnvironment("VISTO_CATALOG_ACTIVE_TTL", 24*time.Hour)
	finishedRefreshTTL := durationEnvironment("VISTO_CATALOG_FINISHED_TTL", 30*24*time.Hour)
	startWorker(func(ctx context.Context) {
		watchService.RunCatalogRefresher(ctx, refreshInterval, activeRefreshTTL, finishedRefreshTTL)
	})
	backupInterval := durationEnvironment("VISTO_BACKUP_INTERVAL", 24*time.Hour)
	backupRetention := durationEnvironment("VISTO_BACKUP_RETENTION", 30*24*time.Hour)
	backupDirectory := environment("VISTO_BACKUP_DIR", filepath.Join(filepath.Dir(databasePath), "backups"))

	encryptionKey := os.Getenv("VISTO_SECRET_ENCRYPTION_KEY")
	var profileConfig profile.PushoverConfig
	var secretCipher *pushover.AESGCMCipher
	if encryptionKey != "" {
		cipher, err := pushover.NewAESGCMCipher(encryptionKey)
		if err != nil {
			log.Fatalf("configure secret encryption: %v", err)
		}
		secretCipher = cipher
		profileConfig.Cipher = cipher
	}
	backupService := &backupjob.Service{Store: store, Cipher: secretCipher, DatabasePath: databasePath, Directory: backupDirectory, DefaultInterval: backupInterval, LocalRetention: backupRetention}
	startWorker(func(ctx context.Context) { backupService.Run(ctx, log.Default()) })
	profiles := profile.NewService(store, profileConfig)
	if secretCipher != nil {
		pushoverClient := pushover.NewClient(nil)
		dispatchInterval := durationEnvironment("VISTO_PUSHOVER_INTERVAL", 15*time.Minute)
		startWorker(func(ctx context.Context) {
			notifications.NewService(store, pushoverClient, secretCipher).Run(ctx, dispatchInterval, log.Default())
		})
	}
	appHandler := http.NewServeMux()
	googleOAuth := httpserver.GoogleOAuthConfig{ClientID: os.Getenv("VISTO_GOOGLE_CLIENT_ID"), ClientSecret: os.Getenv("VISTO_GOOGLE_CLIENT_SECRET"), RedirectURL: os.Getenv("VISTO_GOOGLE_REDIRECT_URL")}
	authService := auth.NewService(store, auth.Config{AllowSignups: boolEnvironment("VISTO_ALLOW_SIGNUPS", true), GoogleEnabled: googleOAuth.Enabled()})
	publicURL := os.Getenv("VISTO_PUBLIC_URL")
	if err := mcpserver.ValidatePublicURL(publicURL); err != nil {
		log.Fatal(err)
	}
	trustedProxies, err := security.ParseTrustedProxies(os.Getenv("VISTO_TRUSTED_PROXY_CIDRS"))
	if err != nil {
		log.Fatal(err)
	}
	trustedProxies = trustedProxies.WithPublicURL(publicURL)
	oauthService := oauth.NewService(store)
	oauthCleanupInterval := durationEnvironment("VISTO_OAUTH_CLEANUP_INTERVAL", 24*time.Hour)
	startWorker(func(ctx context.Context) {
		oauthService.RunCleanup(ctx, oauthCleanupInterval, func(err error) { log.Printf("OAuth cleanup failed: %v", err) })
	})
	activityCleanupInterval := durationEnvironment("VISTO_ACTIVITY_CLEANUP_INTERVAL", 24*time.Hour)
	activityRetention := durationEnvironment("VISTO_ACTIVITY_RETENTION", 365*24*time.Hour)
	startWorker(func(ctx context.Context) {
		activity.NewRetentionService(store).RunCleanup(ctx, activityCleanupInterval, activityRetention, func(removed int, err error) {
			if err != nil {
				log.Printf("activity retention cleanup failed after removing %d events: %v", removed, err)
				return
			}
			log.Printf("activity retention cleanup removed %d events", removed)
		})
	})
	mcpHandler := mcpserver.NewWithTrustedProxies(authService, oauthService, publicURL, metadataProvider, library.NewService(store), tracking.NewService(store), watchService, trustedProxies)
	appHandler.Handle("/mcp", mcpHandler)
	appHandler.Handle("/oauth/", mcpHandler)
	appHandler.Handle("/.well-known/", mcpHandler)
	appServer := httpserver.New(authService, metadataProvider, os.Getenv("VISTO_WEB_DIR"), library.NewService(store), tracking.NewService(store), profiles, feed.NewService(store), exportapp.NewService(store), watchService).
		WithTrustedProxies(trustedProxies).WithOAuth(oauthService).WithGoogleOAuth(googleOAuth).WithBackups(backupService)
	var plexMetadataProvider plexsync.MetadataProvider
	if metadataProvider != nil {
		plexMetadataProvider = metadataProvider
	}
	appServer.WithPlexSync(plexsync.NewService(store, plexMetadataProvider, library.NewService(store), publicURL))
	appHandler.Handle("/", appServer.Handler())
	server := &http.Server{
		Addr:              environment("VISTO_LISTEN_ADDR", ":8080"),
		Handler:           requestLogger(appHandler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Printf("Visto listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve HTTP: %v", err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stopBackground()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown HTTP server: %v", err)
	}
	if !waitForWorkers(ctx, &workers) {
		log.Printf("background workers did not stop before shutdown deadline")
	}
}

func waitForWorkers(ctx context.Context, workers *sync.WaitGroup) bool {
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

func runBackup(sourcePath, destinationPath string) error {
	return sqlite.Backup(context.Background(), sourcePath, destinationPath)
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func boolEnvironment(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("%s must be true or false", name)
	}
	return parsed
}

func durationEnvironment(name string, fallback time.Duration) time.Duration {
	duration, err := parseDuration(name, os.Getenv(name), fallback)
	if err != nil {
		log.Fatal(err)
	}
	return duration
}

func parseDuration(name, value string, fallback time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return duration, nil
}
