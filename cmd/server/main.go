// Package main provides the flagship feature flag service HTTP server.
//
// Application Startup Flow:
//
//  1. Load configuration from environment variables (config.Load)
//  2. Initialize Prometheus metrics registry (telemetry.Init)
//  3. Set rollout salt for deterministic user bucketing (snapshot.SetRolloutSalt)
//  4. Create database store - Postgres or in-memory (store.NewStore)
//  5. Load initial flag snapshot from database (store.GetAllFlags)
//  6. Build and store snapshot in memory (snapshot.BuildFromFlags, snapshot.Update)
//  7. Start API server on :8080 (handles client requests - evaluations, admin ops)
//  8. Start metrics/pprof server on :9090 (for observability - /metrics, /debug/pprof)
//  9. Wait for SIGINT/SIGTERM for graceful shutdown
//  10. Shutdown: close connections, drain audit queue, stop webhook dispatcher
//
// The server runs two HTTP servers concurrently:
//   - API Server (:8080): Client-facing REST API and SSE streaming
//   - Metrics Server (:9090): Prometheus metrics and pprof profiling (internal use)
//
// Graceful Shutdown:
//   Both servers shut down gracefully with a 5-second timeout to allow in-flight
//   requests to complete. The audit service and webhook dispatcher also drain their
//   queues before termination.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	_ "net/http/pprof" // <-- registers /debug/pprof/* on DefaultServeMux
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TimurManjosov/goflagship/internal/api"
	"github.com/TimurManjosov/goflagship/internal/config"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/TimurManjosov/goflagship/internal/telemetry"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Prometheus registry
	telemetry.Init()

	// Set rollout salt for deterministic bucketing
	snapshot.SetRolloutSalt(cfg.RolloutSalt)

	ctx := context.Background()

	// Create store based on configuration
	st, err := store.NewStore(ctx, cfg.StoreType, cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("failed to initialize store (type=%s): %v", cfg.StoreType, err)
	}
	defer st.Close()

	// Load initial flag snapshot into memory
	flags, err := st.GetAllFlags(ctx, cfg.Env)
	if err != nil {
		log.Fatalf("failed to load flags from store: %v", err)
	}
	currentSnapshot := snapshot.BuildFromFlags(flags)
	snapshot.Update(currentSnapshot)
	telemetry.SnapshotFlags.Set(float64(len(currentSnapshot.Flags)))
	log.Printf("[server] snapshot loaded: flags=%d etag=%s store=%s", 
		len(currentSnapshot.Flags), currentSnapshot.ETag, cfg.StoreType)

	// ---- API server (:8080) ----
	apiSrv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      api.NewServer(st, cfg.Env, cfg.AdminAPIKey).Router(),
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 0, // keep SSE connections alive
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Printf("[server] http server listening on %s", cfg.HTTPAddr)
		if err := apiSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("api server: %v", err)
		}
	}()

	// ---- Metrics + pprof server (:9090) ----
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	// forward /debug/pprof/* to DefaultServeMux where pprof registered
	mux.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)

	metricsSrv := &http.Server{
		Addr:         cfg.MetricsAddr,
		Handler:      mux,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Printf("[server] metrics/pprof server listening on %s", cfg.MetricsAddr)
		if err := metricsSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("metrics server: %v", err)
		}
	}()

	// ---- Graceful shutdown for both servers ----
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	log.Println("[server] shutdown signal received, stopping servers...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := apiSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[server] error during API server shutdown: %v", err)
	}
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[server] error during metrics server shutdown: %v", err)
	}

	log.Println("[server] servers stopped successfully")
}
