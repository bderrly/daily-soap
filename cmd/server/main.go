package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/joho/godotenv"

	"derrclan.com/moravian-soap/internal/server"
)

func main() {
	_ = godotenv.Load()
	if err := server.InitDB(); err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewTextHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))

	mux := server.Muxer()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	ctx := context.Background()

	idleConns := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("error shutting down http server", "error", err)
		}
		close(idleConns)
	}()

	slog.Info("starting server", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("http server died", "error", err)
		os.Exit(1)
	}
	<-idleConns
}
