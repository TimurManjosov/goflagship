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
	log.Printf("snapshot loaded: %d flags, etag=%s (store=%s)", 
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
		log.Printf("http listening on %s", cfg.HTTPAddr)
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
		log.Printf("metrics/pprof listening on %s", cfg.MetricsAddr)
		if err := metricsSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("metrics server: %v", err)
		}
	}()

	// ---- Graceful shutdown for both servers ----
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	log.Println("shutdown signal received, stopping servers...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := apiSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("error during API server shutdown: %v", err)
	}
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("error during metrics server shutdown: %v", err)
	}

	log.Println("servers stopped successfully")
}
