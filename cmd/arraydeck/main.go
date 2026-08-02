// Command arraydeck runs the ArrayDeck server: discovery engine, REST API,
// SSE stream and embedded web UI in one process.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gazofnaz/unraid-dashboard/internal/api"
	"github.com/gazofnaz/unraid-dashboard/internal/app"
	"github.com/gazofnaz/unraid-dashboard/internal/config"
)

func main() {
	cfg := config.FromEnv()
	logger := newLogger(cfg.LogLevel)
	logger.Info("arraydeck starting", "version", cfg.Version, "listen", cfg.Listen)

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("startup failed", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go application.Run(ctx)

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           api.New(application, logger),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
	logger.Info("arraydeck stopped")
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
