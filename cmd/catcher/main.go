package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpAdapter "github.com/cwygoda/catcher/internal/adapter/http"
	"github.com/cwygoda/catcher/internal/adapter/processor"
	"github.com/cwygoda/catcher/internal/adapter/sqlite"
	"github.com/cwygoda/catcher/internal/config"
	"github.com/cwygoda/catcher/internal/domain"
	"github.com/cwygoda/catcher/internal/worker"
)

func main() {
	cfg := config.Load()

	log.Printf("starting catcher on port %d", cfg.Port)
	log.Printf("database: %s", cfg.DBPath)
	log.Printf("video dir: %s", cfg.VideoDir)

	// Initialize SQLite repository
	repo, err := sqlite.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer repo.Close()

	// Initialize domain service
	svc := domain.NewJobService(repo)

	// Recover stale jobs from previous crash
	if recovered, err := svc.RecoverStale(context.Background()); err != nil {
		log.Printf("warning: failed to recover stale jobs: %v", err)
	} else if recovered > 0 {
		log.Printf("recovered %d stale jobs", recovered)
	}

	// Initialize processor registry
	registry := processor.NewRegistry()
	registry.Register(processor.NewYouTubeProcessor(cfg.VideoDir))

	// Initialize HTTP server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := httpAdapter.NewServer(svc, addr)

	// Initialize worker
	w := worker.New(svc, registry, cfg.PollInterval, cfg.MaxRetries)

	// Graceful shutdown setup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start worker
	go w.Run(ctx)

	// Start HTTP server
	go func() {
		log.Printf("HTTP server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigCh
	log.Printf("received signal %v, shutting down", sig)

	// Cancel worker context
	cancel()

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("shutdown complete")
}
