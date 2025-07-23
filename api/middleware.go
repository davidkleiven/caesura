package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/davidkleiven/caesura/pkg"
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
				slog.Info("Could not get session", "error", err, "host", r.Host)
				return
			}

			ctx := context.WithValue(r.Context(), sessionKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireMinimumRole(cookieStore *sessions.CookieStore, minimumRole pkg.RoleKind) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := MustGetSession(r)
			data, ok := session.Values["role"].([]byte)
			if !ok {
				http.Error(w, "Value is not slice of bytes", http.StatusBadRequest)
				slog.Info("Role value is not slice of bytes", "host", r.Host)
				return
			}

			var role pkg.UserRole
			if err := json.Unmarshal(data, &role); err != nil {
				http.Error(w, "Could not unmarshal role info", http.StatusBadRequest)
				slog.Info("Could not unmarshal role info", "error", err, "host", r.Host)
				return
			}

			orgId, ok := session.Values["orgId"].(string)
			if !ok {
				http.Error(w, "Could not convert orgId to string", http.StatusBadRequest)
				slog.Info("Could not convert orgId to string", "host", r.Host)
				return
			}

			if orgRole, ok := role.Roles[orgId]; !ok || orgRole < minimumRole {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				slog.Info("User is unauthorized", "host", r.Host, "user", role.UserId)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireRead(cookieStore *sessions.CookieStore) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession),
		RequireMinimumRole(cookieStore, pkg.RoleViewer),
	)
}

func RequireWrite(cookieStore *sessions.CookieStore) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession),
		RequireMinimumRole(cookieStore, pkg.RoleEditor),
	)
}

func RequireAdmin(cookieStore *sessions.CookieStore) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession),
		RequireMinimumRole(cookieStore, pkg.RoleAdmin),
	)
}

func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares); i > 0; i-- {
			final = middlewares[i-1](final)
		}
		return final
	}
}
