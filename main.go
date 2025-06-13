package main

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/davidkleiven/caesura/api"
)

func main() {
	mux := api.Setup()

	port := api.Port()
	slog.Info("Starting server", "port", port)
	log.Fatal(http.ListenAndServe(api.Port(), api.LogRequest(mux)))
	slog.Info("Server stopped")
}
