package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/pixperk/goiler/internal/config"
	"github.com/pixperk/goiler/internal/worker"
	"github.com/pixperk/goiler/pkg/otel"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting worker")

	// Load configuration
	cfg := config.Load()

	// Initialize context
	ctx := context.Background()

	// Initialize OpenTelemetry
	tracerProvider, err := otel.NewTracerProvider(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize tracer", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer tracerProvider.Shutdown(ctx)

	// Create worker server
	srv := worker.NewServer(cfg, logger)

	// Handle shutdown signals
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("shutting down worker")
		srv.Shutdown()
	}()

	// Start worker server
	if err := srv.Start(); err != nil {
		logger.Error("worker error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
