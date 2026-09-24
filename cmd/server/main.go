package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	exportapp "github.com/afonsocosta/visto/internal/application/export"
	"github.com/afonsocosta/visto/internal/application/feed"
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/profile"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/application/watch"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	"github.com/afonsocosta/visto/internal/infrastructure/tmdb"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
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

	var metadataProvider *tmdb.Client
	if apiKey := os.Getenv("VISTO_TMDB_API_KEY"); apiKey != "" {
		metadataProvider, err = tmdb.New(apiKey, nil)
		if err != nil {
			log.Fatalf("configure TMDB: %v", err)
		}
	}
	watchService := watch.NewService(store, metadataProvider)
	refreshContext, stopRefresh := context.WithCancel(context.Background())
	defer stopRefresh()
	go watchService.RunCatalogRefresher(refreshContext, 6*time.Hour, 24*time.Hour, 30*24*time.Hour)
	server := &http.Server{
		Addr:              environment("VISTO_LISTEN_ADDR", ":8080"),
		Handler:           httpserver.New(auth.NewService(store), metadataProvider, os.Getenv("VISTO_WEB_DIR"), library.NewService(store), tracking.NewService(store), profile.NewService(store), feed.NewService(store), exportapp.NewService(store), watchService).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
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
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown HTTP server: %v", err)
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
