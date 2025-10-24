package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TimurManjosov/goflagship/internal/api"
	"github.com/TimurManjosov/goflagship/internal/config"
	mydb "github.com/TimurManjosov/goflagship/internal/db"
	"github.com/TimurManjosov/goflagship/internal/repo"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
)

func main() {
	cfg, err := config.Load()
	if err != nil { log.Fatalf("config: %v", err) }

	ctx := context.Background()
	pool, err := mydb.NewPool(ctx, cfg.DB_DSN)
	if err != nil { log.Fatalf("db: %v", err) }
	defer pool.Close()

	rp := repo.New(pool)

	// initial snapshot
	rows, err := rp.GetAllFlags(ctx, cfg.Env)
	if err != nil { log.Fatalf("load flags: %v", err) }
	s := snapshot.BuildFromRows(rows)
	snapshot.Update(s)
	log.Printf("snapshot: %d flags, etag=%s", len(s.Flags), s.ETag)

	// API server with deps
	srvAPI := api.NewServer(rp, cfg.Env, cfg.AdminAPIKey)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      srvAPI.Router(),
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctxShut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctxShut)
	log.Println("stopped")
}
