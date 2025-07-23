package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"
)

func LogRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request method and URL
		method := r.Method
		url := r.URL.String()

		acceptEncoding := r.Header.Get("Accept-Encoding")
		acceptHeaders := r.Header.Get("Accept")
		// You can replace this with your logging mechanism
		slog.Info("Received request", "method", method, "url", url, "accept", acceptHeaders, "accept-encoding", acceptEncoding, "host", r.Host)

		// Call the next handler in the chain
		handler.ServeHTTP(w, r)
	})
}

func RequireSession(cookieStore *sessions.CookieStore, name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := cookieStore.Get(r, name)
			if err != nil || session == nil {
				http.Error(w, "Could not get session "+err.Error(), http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), sessionKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
