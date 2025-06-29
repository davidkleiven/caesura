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
	"github.com/davidkleiven/caesura/pkg"
)

func main() {
	config := pkg.NewDefaultConfig()
	cfgFile, ok := os.LookupEnv("CAESURA_CONFIG")

	if ok {
		var err error
		config, err = pkg.OverrideFromFile(cfgFile, config)
		if err != nil {
			slog.Error("Failed to load configuration from file", "file", cfgFile, "error", err)
			os.Exit(1)
		}
	}

	if err := config.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	storeMng := api.StoreManager{
		Store: pkg.GetStore(config),
	}
	mux := api.Setup(&storeMng)
	port := api.Port()

	server := &http.Server{
		Addr:    port,
		Handler: api.LogRequest(mux),
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("Starting server", "port", port)
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
