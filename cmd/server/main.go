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

	ctx := context.Background()

	// Create store based on configuration
	st, err := store.NewStore(ctx, cfg.StoreType, cfg.DB_DSN)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	// initial snapshot
	flags, err := st.GetAllFlags(ctx, cfg.Env)
	if err != nil {
		log.Fatalf("load flags: %v", err)
	}
	s := snapshot.BuildFromFlags(flags)
	snapshot.Update(s)
	telemetry.SnapshotFlags.Set(float64(len(s.Flags)))
	log.Printf("snapshot: %d flags, etag=%s (store_type=%s)", len(s.Flags), s.ETag, cfg.StoreType)

	// ---- API server (:8080) ----
	apiSrv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      api.NewServer(st, cfg.Env, cfg.AdminAPIKey).Router(),
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 0,                 // keep SSE connections alive
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

	// ---- graceful shutdown for both servers ----
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctxShut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = apiSrv.Shutdown(ctxShut)
	_ = metricsSrv.Shutdown(ctxShut)

	log.Println("stopped")
}
