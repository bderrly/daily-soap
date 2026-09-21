// Package main is the entry point for the daily-soap server.
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

	"github.com/joho/godotenv"

	"github.com/bderrly/daily-soap/internal/server"

	_ "time/tzdata"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	_ = godotenv.Load()

	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		slog.Warn("invalid LOG_LEVEL, defaulting to INFO", "level", os.Getenv("LOG_LEVEL"), "error", err)
	}

	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))

	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func parseLogLevel(s string) (slog.Level, error) {
	if s == "" {
		return slog.LevelInfo, nil
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo, fmt.Errorf("unmarshaling log level: %w", err)
	}
	return lvl, nil
}

func run() error {
	_ = godotenv.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appStore, err := server.InitDB(ctx)
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	app, err := server.NewApplication(appStore)
	if err != nil {
		return fmt.Errorf("creating application: %w", err)
	}

	mux := app.Routes()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	idleConns := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		slog.Info("shutting down server...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutting down http server", "error", err)
		}
		close(idleConns)
	}()

	slog.Info("starting server", slog.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen and serve: %w", err)
	}
	<-idleConns
	return nil
}
