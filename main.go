package main

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/davidkleiven/caesura/api"
	"github.com/davidkleiven/caesura/web"
)

func main() {
	http.HandleFunc("/", api.RootHandler)
	http.Handle("/css/", web.CssServer())
	http.HandleFunc("/instruments", api.InstrumentSearchHandler)
	http.HandleFunc("/choice", api.ChoiceHandler)

	port := api.Port()
	slog.Info("Starting server", "port", port)
	log.Fatal(http.ListenAndServe(api.Port(), api.LogRequest(http.DefaultServeMux)))
	slog.Info("Server stopped")
}
