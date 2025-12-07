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

	"github.com/davidkleiven/caesura/api"
	"github.com/davidkleiven/caesura/pkg"
	"github.com/gorilla/sessions"
	"github.com/stripe/stripe-go/v84"
)

func main() {
	profile := os.Getenv("CAESURA_PROFILE") // test
	handler := pkg.NewHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	ctxLogger := slog.New(handler)
	slog.SetDefault(ctxLogger)

	config, err := pkg.LoadProfile(fmt.Sprintf("config-%s.yml", profile))
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := config.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	storeResult := pkg.GetStore(config)
	if storeResult.Err != nil {
		slog.Error("Store initialization failed", "error", storeResult.Err)
		os.Exit(1)
	}
	defer storeResult.Cleanup()

	cookieStore := sessions.NewCookieStore([]byte(config.CookieSecretSignKey))
	mux := api.Setup(storeResult.Store, config, cookieStore)
	stripe.Key = config.StripeSecretKey

	rateLimiter := api.NewRateLimiter(config.MaxNumRequestsPerMinute, time.Minute)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: rateLimiter.Middleware(api.LogRequest(mux)),
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	cancelCtx, doCancel := context.WithCancel(context.Background())
	defer doCancel()

	go func() {
		slog.Info("Starting server", "port", config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
		}
	}()

	go func(ctx context.Context) {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rateLimiter.Cleanup()
			case <-ctx.Done():
				slog.Info("Stopping rate limiter cleanup")
				return
			}
		}
	}(cancelCtx)

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
