package logger

import (
	"io"
	"log/slog"
	"os"

	"github.com/hrvtonn/ecommerce-devops-pipeline/internal/config"
)

// New builds the application logger: JSON at info level in production,
// human-readable text at debug level otherwise, and silent under tests.
func New(cfg config.Config) *slog.Logger {
	var handler slog.Handler
	switch {
	case cfg.IsTest():
		handler = slog.NewTextHandler(io.Discard, nil)
	case cfg.IsProduction():
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	default:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	return slog.New(handler).With("service", "ecommerce-api")
}
