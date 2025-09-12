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

func RequireSession(cookieStore *sessions.CookieStore, name string, opts *sessions.Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := cookieStore.Get(r, name)
			if err != nil || session == nil {
				http.Error(w, "Could not get session "+err.Error(), http.StatusInternalServerError)
				slog.Info("Could not get session", "error", err, "host", r.Host)
				return
			}

			session.Options = opts
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

			var role pkg.UserInfo
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
				slog.Info("User is unauthorized", "host", r.Host, "user", role.Id, "role", orgRole, "required-role", minimumRole, "role-provided", ok, "organization-id", orgId)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireUserId(cookieStore *sessions.CookieStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := MustGetSession(r)
			_, ok := session.Values["userId"].(string)
			if !ok {
				http.Error(w, "User id is not present", http.StatusBadRequest)
				slog.Info("User id is not present", "host", r.Host)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ValidateUserInfo(cookieStore *sessions.CookieStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := MustGetSession(r)
			data, ok := session.Values["role"].([]byte)
			if !ok {
				http.Error(w, "User role is not present", http.StatusBadRequest)
				slog.Info("User role is not present", "host", r.Host)
				return
			}

			var info pkg.UserInfo
			if err := json.Unmarshal(data, &info); err != nil {
				http.Error(w, "Could not interpret role data", http.StatusBadRequest)
				slog.Error("Could not interpret role data", "error", err, "host", r.Host)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireWriteSubscription(config *pkg.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.RequireSubscription {
				session := MustGetSession(r)
				canWrite, ok := session.Values[SubscriptionWriteAllowed].(bool)
				if !canWrite || !ok {
					http.Error(w, "Subscription expired", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireRead(cookieStore *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession, opts),
		RequireMinimumRole(cookieStore, pkg.RoleViewer),
	)
}

func RequireWrite(config *pkg.Config, cookieStore *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession, opts),
		RequireWriteSubscription(config),
		RequireMinimumRole(cookieStore, pkg.RoleEditor),
	)
}

func RequireAdmin(config *pkg.Config, cookieStore *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession, opts),
		RequireWriteSubscription(config),
		RequireMinimumRole(cookieStore, pkg.RoleAdmin),
	)
}

func RequireAdminWithoutSubscription(cookieStore *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession, opts),
		RequireMinimumRole(cookieStore, pkg.RoleAdmin),
	)
}

func RequireSignedIn(cookieStore *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession, opts),
		RequireUserId(cookieStore),
	)
}
func RequireUserInfo(cookieStore *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession, opts),
		RequireUserId(cookieStore),
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
