package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry        *prometheus.Registry
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
}

// New creates a registry with the HTTP metrics plus the default Go runtime and
// process collectors (CPU, memory, goroutines, GC).
func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.3, 0.5, 1, 2, 5},
		}, []string{"method", "route", "status_code"}),
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "route", "status_code"}),
	}

	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.requestDuration,
		m.requestsTotal,
	)
	return m
}

// Middleware records the count and latency of every request, labelled by the
// matched route pattern (e.g. /api/v1/products/{id}).
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		route := "unmatched"
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			route = rctx.RoutePattern()
		}
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		labels := prometheus.Labels{"method": r.Method, "route": route, "status_code": strconv.Itoa(status)}
		m.requestDuration.With(labels).Observe(time.Since(start).Seconds())
		m.requestsTotal.With(labels).Inc()
	})
}

// Handler serves the Prometheus scrape endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
