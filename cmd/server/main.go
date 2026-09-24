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
	"github.com/afonsocosta/visto/internal/application/library"
	"github.com/afonsocosta/visto/internal/application/profile"
	"github.com/afonsocosta/visto/internal/application/tracking"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
	"github.com/afonsocosta/visto/internal/infrastructure/tmdb"
	httpserver "github.com/afonsocosta/visto/internal/presentation/http"
)

func main() {
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
	server := &http.Server{
		Addr:              environment("VISTO_LISTEN_ADDR", ":8080"),
		Handler:           httpserver.New(auth.NewService(store), metadataProvider, os.Getenv("VISTO_WEB_DIR"), library.NewService(store), tracking.NewService(store), profile.NewService(store)).Handler(),
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

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
