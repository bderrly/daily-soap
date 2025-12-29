package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"derrclan.com/moravian-soap/internal/server"
)

func main() {
	mux := server.Muxer()

	srv := http.Server{
		Addr:    ":42069",
		Handler: mux,
	}

	ctx := context.Background()

	idleConns := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("error shutting down http server")
		}
		close(idleConns)
	}()

	if err := srv.ListenAndServe(); err != nil {
		slog.Error("http server died")
	}
	<-idleConns
}
