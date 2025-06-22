package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidkleiven/caesura/api"
	"github.com/davidkleiven/caesura/config"
)

func main() {
	configuration := config.NewDefaultConfig()
	if filename, ok := os.LookupEnv("CAESURA_CONFIG"); ok {
		file, err := os.Open(filename)
		if err != nil {
			slog.Error("Failed to open configuration file", "error", err)
			os.Exit(1)
		}
		defer file.Close()
		if err := config.UpdateFromReader(configuration, file); err != nil {
			slog.Error("Failed to load configuration", "error", err)
			os.Exit(1)
		}
	}
	config.UpdateFromEnv(configuration)

	imageHandler := api.NewImageHandler(api.WithConfig(configuration))
	mux := api.Setup(imageHandler)

	server := &http.Server{
		Addr:    configuration.Port,
		Handler: api.LogRequest(mux),
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("Starting server", "port", configuration.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
		}
	}()

	<-stop
	slog.Info("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 5.0*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	} else {
		slog.Info("Server gracefully stopped")
	}
}
