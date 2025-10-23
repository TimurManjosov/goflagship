package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(middleware.Timeout(5 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Get("/v1/flags/snapshot", func(w http.ResponseWriter, req *http.Request) {
		s := snapshot.Load()
		if inm := req.Header.Get("If-None-Match"); inm != "" && inm == s.ETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", s.ETag)
		_ = json.NewEncoder(w).Encode(s)
	})

	return r
}
