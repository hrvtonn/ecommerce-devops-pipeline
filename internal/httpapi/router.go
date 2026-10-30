// Package httpapi wires the HTTP routes, middlewares and handlers.
package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/config"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/docs"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/metrics"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/service"
)

type Services struct {
	Categories *service.CategoryService
	Products   *service.ProductService
	Customers  *service.CustomerService
	Orders     *service.OrderService
}

func NewRouter(cfg config.Config, svc Services, m *metrics.Metrics) http.Handler {
	startedAt := time.Now()
	r := chi.NewRouter()

	r.Use(recoverer)
	r.Use(securityHeaders)
	r.Use(cors)
	r.Use(m.Middleware)
	if !cfg.IsTest() {
		r.Use(newRateLimiter(cfg.RateLimit.Window, cfg.RateLimit.Max, "/health", "/metrics").Middleware)
	}

	// Swagger UI
	r.Get("/api-docs", docs.UI)
	r.Get("/api-docs/", docs.UI)
	r.Get("/api-docs.json", docs.Spec)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"uptime":    time.Since(startedAt).Seconds(),
		})
	})

	// Prometheus scrape endpoint
	r.Method(http.MethodGet, "/metrics", m.Handler())

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		categories := categoryHandler{svc.Categories}
		r.Route("/categories", func(r chi.Router) {
			r.Get("/", handle(categories.findAll))
			r.Post("/", handle(categories.create))
			r.Get("/{id}", handle(categories.findByID))
			r.Put("/{id}", handle(categories.update))
			r.Delete("/{id}", handle(categories.delete))
		})

		products := productHandler{svc.Products}
		r.Route("/products", func(r chi.Router) {
			r.Get("/", handle(products.findAll))
			r.Post("/", handle(products.create))
			r.Get("/{id}", handle(products.findByID))
			r.Put("/{id}", handle(products.update))
			r.Delete("/{id}", handle(products.delete))
		})

		customers := customerHandler{svc.Customers}
		r.Route("/customers", func(r chi.Router) {
			r.Get("/", handle(customers.findAll))
			r.Post("/", handle(customers.create))
			r.Get("/{id}", handle(customers.findByID))
			r.Put("/{id}", handle(customers.update))
			r.Delete("/{id}", handle(customers.delete))
		})

		orders := orderHandler{svc.Orders}
		r.Route("/orders", func(r chi.Router) {
			r.Get("/", handle(orders.findAll))
			r.Post("/", handle(orders.create))
			r.Get("/{id}", handle(orders.findByID))
			r.Patch("/{id}/status", handle(orders.updateStatus))
		})
	})

	// 404 handler for unmatched routes
	r.NotFound(notFound)
	r.MethodNotAllowed(notFound)

	return r
}

func notFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, errorBody{
		Status:  "error",
		Code:    "ROUTE_NOT_FOUND",
		Message: fmt.Sprintf("Route %s %s not found", r.Method, r.URL.RequestURI()),
	})
}
