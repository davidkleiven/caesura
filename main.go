package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/davidkleiven/caesura/api"
	"github.com/davidkleiven/caesura/pkg"
	"github.com/gorilla/sessions"
	"github.com/stripe/stripe-go/v82"
)

func main() {
	cfgFile := os.Getenv("CAESURA_CONFIG")
	handler := pkg.NewHandler(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	ctxLogger := slog.New(handler)
	slog.SetDefault(ctxLogger)

	config, err := pkg.LoadConfig(cfgFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	if config.StoreType == pkg.GoogleCloud {
		backgroundCtx := context.Background()
		googleConfig := pkg.LoadGoogleConfig()
		firestoreClient, err := firestore.NewClient(backgroundCtx, googleConfig.ProjectId)
		if err != nil {
			slog.Error("Failed to create firestore client", "error", err)
			os.Exit(1)
		}
		defer firestoreClient.Close()

		cloudStoreClient, err := storage.NewClient(backgroundCtx)
		if err != nil {
			slog.Error("Failed to create cloud storage client", "error", err)
			os.Exit(1)
		}
		defer cloudStoreClient.Close()

		config.GoogleClients.FirestoreClient = firestoreClient
		config.GoogleClients.CloudStoreClient = cloudStoreClient
		config.GoogleCfg = *googleConfig
	}

	if err := config.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	cookieStore := sessions.NewCookieStore([]byte(config.CookieSecretSignKey))
	mux := api.Setup(pkg.GetStore(config), config, cookieStore)
	stripe.Key = config.StripeSecretKey

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: api.LogRequest(mux),
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("Starting server", "port", config.Port)
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
