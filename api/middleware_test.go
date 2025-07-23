package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/gorilla/sessions"
)

func TestLogHandler(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	logHandler := LogRequest(handler)
	buffer := bytes.NewBufferString("")
	origLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buffer, &slog.HandlerOptions{})))
	defer slog.SetDefault(origLogger)

	body := bytes.NewBuffer([]byte{})
	request := httptest.NewRequest("GET", "http://example.com/test", body)

	writer := httptest.NewRecorder()
	logHandler.ServeHTTP(writer, request)

	if writer.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", writer.Code)
		return
	}

	if !strings.Contains(buffer.String(), "GET") {
		t.Error("Expected log to contain 'Received request'")
		return
	}
}

func TestHandleGoogleLoginInternalErrorWrongSession(t *testing.T) {
	cookie := sessions.NewCookieStore([]byte("some-secret-key"))
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/login", nil)
	request.AddCookie(&http.Cookie{
		Name:  AuthSession,
		Value: "CorruptedCookieValue",
	})

	middleware := RequireSession(cookie, AuthSession)
	middleware(handler).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusInternalServerError, recorder.Code)
	}

	if called {
		t.Fatal("Handler should not be called")
	}
}

func TestRequireminimumRoleNoBytes(t *testing.T) {
	validJson := `{"userId": "aaa", "roles": {"org1": 1}}`
	validJsonReadOnly := `{"userId": "aaa", "roles": {"org1": 0}}`
	for _, test := range []struct {
		sessionModifier func(session *sessions.Session)
		desc            string
		code            int
	}{
		{
			sessionModifier: func(s *sessions.Session) {},
			desc:            "Nothing in the session",
			code:            http.StatusBadRequest,
		},
		{
			sessionModifier: func(s *sessions.Session) {
				s.Values["role"] = "not a byte array"
			},
			desc: "Wrong type",
			code: http.StatusBadRequest,
		},
		{
			sessionModifier: func(s *sessions.Session) {
				s.Values["role"] = []byte("not a valid json")
			},
			desc: "Wrong type",
			code: http.StatusBadRequest,
		},
		{
			sessionModifier: func(s *sessions.Session) {
				s.Values["role"] = []byte(validJson)
			},
			desc: "Missing oranization Id",
			code: http.StatusBadRequest,
		},
		{
			sessionModifier: func(s *sessions.Session) {
				s.Values["role"] = []byte(validJson)
				s.Values["orgId"] = "org1"
			},
			desc: "Everything OK",
			code: http.StatusOK,
		},
		{
			sessionModifier: func(s *sessions.Session) {
				s.Values["role"] = []byte(validJsonReadOnly)
				s.Values["orgId"] = "org1"
			},
			desc: "Unauthorized",
			code: http.StatusUnauthorized,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {

			cookie := sessions.NewCookieStore([]byte("key"))
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := RequireMinimumRole(cookie, pkg.RoleEditor)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "/route", nil)
			session, err := cookie.Get(request, "some-session")
			if err != nil {
				t.Fatal(err)
			}
			test.sessionModifier(session)

			ctx := context.WithValue(request.Context(), sessionKey, session)
			middleware(handler).ServeHTTP(recorder, request.WithContext(ctx))

			if recorder.Code != test.code {
				t.Fatalf("Wanted return code to be '%d' got '%d'", test.code, recorder.Code)
			}
		})
	}
}

func TestAccessMiddleware(t *testing.T) {
	for _, test := range []struct {
		middleware func(cookie *sessions.CookieStore) func(http.Handler) http.Handler
		role       pkg.RoleKind
		code       int
		desc       string
	}{
		{
			middleware: RequireRead,
			role:       pkg.RoleViewer,
			code:       http.StatusOK,
			desc:       "Require read, have read",
		},
		{
			middleware: RequireRead,
			role:       pkg.RoleEditor,
			code:       http.StatusOK,
			desc:       "Reader read, have write",
		},
		{
			middleware: RequireRead,
			role:       pkg.RoleAdmin,
			code:       http.StatusOK,
			desc:       "Reader read, have admin",
		},
		{
			middleware: RequireWrite,
			role:       pkg.RoleViewer,
			code:       http.StatusUnauthorized,
			desc:       "Require write, have read",
		},
		{
			middleware: RequireWrite,
			role:       pkg.RoleEditor,
			code:       http.StatusOK,
			desc:       "Reader write, have write",
		},
		{
			middleware: RequireWrite,
			role:       pkg.RoleAdmin,
			code:       http.StatusOK,
			desc:       "Reader write, have admin",
		},
		{
			middleware: RequireAdmin,
			role:       pkg.RoleViewer,
			code:       http.StatusUnauthorized,
			desc:       "Require admin, have read",
		},
		{
			middleware: RequireAdmin,
			role:       pkg.RoleEditor,
			code:       http.StatusUnauthorized,
			desc:       "Reader admin, have write",
		},
		{
			middleware: RequireAdmin,
			role:       pkg.RoleAdmin,
			code:       http.StatusOK,
			desc:       "Reader admin, have admin",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			cookie := sessions.NewCookieStore([]byte("key"))
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "/route", nil)
			session, err := cookie.Get(request, AuthSession)
			if err != nil {
				t.Fatal(err)
			}

			ordId := "orgId"
			userRoles := pkg.UserRole{
				UserId: "aaa",
				Roles: map[string]pkg.RoleKind{
					ordId: test.role,
				},
			}

			data, err := json.Marshal(userRoles)
			if err != nil {
				t.Fatal(err)
			}

			session.Values["role"] = data
			session.Values["orgId"] = ordId

			middleware := test.middleware(cookie)

			ctx := context.WithValue(request.Context(), sessionKey, session)
			middleware(handler).ServeHTTP(recorder, request.WithContext(ctx))

			if recorder.Code != test.code {
				t.Fatalf("Wanted '%d' got '%d'", test.code, recorder.Code)
			}
		})
	}
}

func TestAccessMiddlewareBadRequestOnMissingSession(t *testing.T) {
	cookie := sessions.NewCookieStore([]byte("key"))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	for i, middleware := range []func(http.Handler) http.Handler{
		RequireRead(cookie),
		RequireWrite(cookie),
		RequireAdmin(cookie),
	} {
		t.Run(fmt.Sprintf("Test: #%d", i), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "/endpoint", nil)
			wrappedHandler := middleware(handler)
			wrappedHandler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("Wanted '%d' got '%d'", http.StatusBadRequest, recorder.Code)
			}

			text := recorder.Body.String()

			if !strings.Contains(text, "slice of bytes") {
				t.Fatalf("Wanted body to contain 'slice of bytes' got '%s'", text)
			}
		})

	}
}
