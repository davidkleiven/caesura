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

		// Populate context with meta information
		ctx := context.WithValue(r.Context(), pkg.ReqIdKey, pkg.RandomInsecureID())
		ctx = context.WithValue(ctx, pkg.HostKey, r.Host)

		// You can replace this with your logging mechanism
		slog.InfoContext(ctx, "Received request", "method", method, "url", url, "accept", acceptHeaders, "accept-encoding", acceptEncoding)

		// Call the next handler in the chain
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireSession(cookieStore *sessions.CookieStore, name string, opts *sessions.Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := cookieStore.Get(r, name)
			if err != nil && session == nil {
				slog.ErrorContext(r.Context(), "Could not get session", "error", err)
				http.Error(w, "Failed to create fresh cookie: "+err.Error(), http.StatusInternalServerError)
				return
			} else if err != nil {
				// Session is not nil
				slog.InfoContext(r.Context(), "New session created", "error", err)
				if err := session.Save(r, w); err != nil {
					slog.ErrorContext(r.Context(), "Failed to save new session", "error", err)
					http.Error(w, "Failed to save session", http.StatusInternalServerError)
					return
				}
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
				slog.InfoContext(r.Context(), "Role value is not slice of bytes")
				return
			}

			var role pkg.UserInfo
			if err := json.Unmarshal(data, &role); err != nil {
				http.Error(w, "Could not unmarshal role info", http.StatusBadRequest)
				slog.InfoContext(r.Context(), "Could not unmarshal role info", "error", err)
				return
			}

			orgId, ok := session.Values["orgId"].(string)
			if !ok {
				http.Error(w, "Could not convert orgId to string", http.StatusBadRequest)
				slog.InfoContext(r.Context(), "Could not convert orgId to string")
				return
			}

			// Provide context
			ctx := context.WithValue(r.Context(), pkg.UserIdKey, role.Id)
			ctx = context.WithValue(ctx, pkg.OrgIdKey, orgId)

			if orgRole, ok := role.Roles[orgId]; !ok || orgRole < minimumRole {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				slog.InfoContext(ctx, "User is unauthorized", "role", orgRole, "required-role", minimumRole, "role-provided", ok)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
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
				slog.InfoContext(r.Context(), "User id is not present")
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
				slog.InfoContext(r.Context(), "User role is not present")
				return
			}

			var info pkg.UserInfo
			if err := json.Unmarshal(data, &info); err != nil {
				http.Error(w, "Could not interpret role data", http.StatusBadRequest)
				slog.ErrorContext(r.Context(), "Could not interpret role data", "error", err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func trySaveSession(session *sessions.Session, r *http.Request, w http.ResponseWriter) {
	// Update session with subscription information
	if err := session.Save(r, w); err != nil {
		slog.ErrorContext(r.Context(), "Failed to store subscription information in session. Proceeding with request anyways", "error", err)
	}
}

func RequireWriteSubscription(store pkg.SubscriptionValidator, config *pkg.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.RequireSubscription {
				session := MustGetSession(r)
				canWrite, ok := session.Values[SubscriptionWriteAllowed].(bool)
				if !ok {
					slog.InfoContext(r.Context(), "No subscription info in context. Attempting to collect information")
					// No information about subscriptions, try to collect subscription information
					subCollector := SubscriptionHandler{store: store, timeout: config.Timeout}
					orgId, hasOrgId := session.Values["orgId"].(string)
					if !hasOrgId || orgId == "" {
						slog.InfoContext(r.Context(), "Attempted to request subscription without having an organization")
						http.Error(w, "No information about organization available", http.StatusForbidden)
						return
					}

					ctx, cancel := context.WithTimeout(r.Context(), config.Timeout)
					defer cancel()

					result := subCollector.GetInfo(ctx, orgId)
					result.PopulateSession(session)
					canWrite = MustGetCanWrite(session)
					trySaveSession(session, r, w)
				}
				if !canWrite {
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

func RequireWrite(store pkg.SubscriptionValidator, config *pkg.Config, cookieStore *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession, opts),
		RequireWriteSubscription(store, config),
		RequireMinimumRole(cookieStore, pkg.RoleEditor),
	)
}

func RequireAdmin(store pkg.SubscriptionValidator, config *pkg.Config, cookieStore *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
	return Chain(
		RequireSession(cookieStore, AuthSession, opts),
		RequireWriteSubscription(store, config),
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
