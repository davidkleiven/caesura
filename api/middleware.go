package api

import (
	"log/slog"
	"net/http"
)

func LogRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request method and URL
		method := r.Method
		url := r.URL.String()

		acceptEncoding := r.Header.Get("Accept-Encoding")
		acceptHeaders := r.Header.Get("Accept")
		// You can replace this with your logging mechanism
		slog.Info("Received request", "method", method, "url", url, "accept", acceptHeaders, "accept-encoding", acceptEncoding)

		// Call the next handler in the chain
		handler.ServeHTTP(w, r)
	})
}
