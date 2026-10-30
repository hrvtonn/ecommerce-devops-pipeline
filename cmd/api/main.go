// Command api runs the e-commerce API.
//
// Usage:
//
//	ecommerce-api [serve]     start the HTTP server (runs migrations first)
//	ecommerce-api migrate     apply database migrations
//	ecommerce-api seed        populate sample data
//	ecommerce-api healthcheck probe GET /health (used by the Docker HEALTHCHECK)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/cache"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/config"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/database"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/httpapi"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/logger"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/metrics"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/repository"
	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/service"
)

const shutdownTimeout = 10 * time.Second

func main() {
	cfg := config.Load()
	slog.SetDefault(logger.New(cfg))

	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch command {
	case "serve":
		err = serve(ctx, cfg)
	case "migrate":
		err = withPool(ctx, cfg, database.Migrate)
	case "seed":
		err = withPool(ctx, cfg, database.Seed)
	case "healthcheck":
		err = healthcheck(cfg)
	default:
		err = fmt.Errorf("unknown command %q (expected serve, migrate, seed or healthcheck)", command)
	}

	if err != nil {
		slog.Error("Command failed", "command", command, "error", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, cfg config.Config) error {
	pool, err := database.NewPool(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer func() {
		pool.Close()
		slog.Info("Database pool closed")
	}()

	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}
	slog.Info("Database migrations applied")

	redisClient := cache.NewRedisClient(cfg)
	if redisClient != nil {
		if err := redisClient.Ping(ctx).Err(); err != nil {
			slog.Error("Failed to connect to Redis", "error", err)
		} else {
			slog.Info("Redis connected")
		}
		defer func() {
			if err := redisClient.Close(); err != nil {
				slog.Error("Error disconnecting Redis", "error", err)
				return
			}
			slog.Info("Redis disconnected")
		}()
	}

	c := cache.New(redisClient)
	categories := repository.NewCategoryRepository(pool)
	products := repository.NewProductRepository(pool)
	customers := repository.NewCustomerRepository(pool)
	orders := repository.NewOrderRepository(pool)

	router := httpapi.NewRouter(cfg, httpapi.Services{
		Categories: service.NewCategoryService(categories, c),
		Products:   service.NewProductService(products, categories, c),
		Customers:  service.NewCustomerService(customers, c),
		Orders:     service.NewOrderService(orders, products, customers, database.NewTransactor(pool), c),
	}, metrics.New())

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info(fmt.Sprintf("Server running on http://localhost:%d", cfg.Port))
		slog.Info("Environment: " + cfg.Env)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	// Graceful shutdown
	slog.Info("Shutdown signal received. Shutting down gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("forced shutdown after timeout: %w", err)
	}
	slog.Info("HTTP server closed")
	return nil
}

func withPool(ctx context.Context, cfg config.Config, fn func(context.Context, *pgxpool.Pool) error) error {
	pool, err := database.NewPool(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	return fn(ctx, pool)
}

func healthcheck(cfg config.Config) error {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", cfg.Port))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("unhealthy: status " + resp.Status)
	}
	return nil
}
