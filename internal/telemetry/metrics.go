package telemetry

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpReqs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"route", "method", "status"},
	)
	httpDur = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route", "method"},
	)

	SSEClients = prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "sse_clients",
    Help: "Number of currently connected SSE clients",
	})
  SnapshotFlags = prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "snapshot_flags",
    Help: "Number of flags currently in the in-memory snapshot",
	})

)

func Init() {
	prometheus.MustRegister(httpReqs, httpDur, SSEClients, SnapshotFlags)
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get route pattern if available
		route := r.URL.Path
		if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
			route = rc.RoutePattern()
		}

		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r)

		httpReqs.WithLabelValues(route, r.Method, http.StatusText(ww.status)).Inc()
		httpDur.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
