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
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
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
		t.Fatalf("Expected status code 200, got %d", writer.Code)

	}

	if !strings.Contains(buffer.String(), "GET") {
		t.Fatal("Expected log to contain 'Received request'")

	}
}

func TestHandleGoogleLoginInternalErrorWrongSession(t *testing.T) {
	cookie := sessions.NewCookieStore([]byte("some-secret-key"))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/login", nil)
	request.AddCookie(&http.Cookie{
		Name:  AuthSession,
		Value: "CorruptedCookieValue",
	})

	opt := sessions.Options{}
	middleware := RequireSession(cookie, AuthSession, &opt)
	middleware(handler).ServeHTTP(recorder, request)

	setCookie := recorder.Header().Get("Set-Cookie")
	testutils.AssertContains(t, setCookie, AuthSession, "=")

	if strings.Contains(setCookie, "Corrputed") {
		t.Fatalf("%s contains 'Corrputed'", setCookie)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusOK, recorder.Code)
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
			desc: "Invalid role",
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
	opt := sessions.Options{}

	readWithConfig := func(config *pkg.Config, cookie *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
		_ = config
		return RequireRead(cookie, opts)
	}

	adminWithoutSub := func(config *pkg.Config, cookie *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
		_ = config
		return RequireAdminWithoutSubscription(cookie, opts)
	}

	writeWithStore := func(config *pkg.Config, cookie *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
		store := pkg.NewMultiOrgInMemoryStore()
		return RequireWrite(store, config, cookie, opts)
	}

	adminWithStore := func(config *pkg.Config, cookie *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler {
		store := pkg.NewMultiOrgInMemoryStore()
		return RequireAdmin(store, config, cookie, opts)
	}

	for _, test := range []struct {
		middleware func(config *pkg.Config, cookie *sessions.CookieStore, opts *sessions.Options) func(http.Handler) http.Handler
		role       pkg.RoleKind
		code       int
		desc       string
	}{
		{
			middleware: readWithConfig,
			role:       pkg.RoleViewer,
			code:       http.StatusOK,
			desc:       "Require read, have read",
		},
		{
			middleware: readWithConfig,
			role:       pkg.RoleEditor,
			code:       http.StatusOK,
			desc:       "Reader read, have write",
		},
		{
			middleware: readWithConfig,
			role:       pkg.RoleAdmin,
			code:       http.StatusOK,
			desc:       "Reader read, have admin",
		},
		{
			middleware: writeWithStore,
			role:       pkg.RoleViewer,
			code:       http.StatusUnauthorized,
			desc:       "Require write, have read",
		},
		{
			middleware: writeWithStore,
			role:       pkg.RoleEditor,
			code:       http.StatusOK,
			desc:       "Reader write, have write",
		},
		{
			middleware: writeWithStore,
			role:       pkg.RoleAdmin,
			code:       http.StatusOK,
			desc:       "Reader write, have admin",
		},
		{
			middleware: adminWithStore,
			role:       pkg.RoleViewer,
			code:       http.StatusUnauthorized,
			desc:       "Require admin, have read",
		},
		{
			middleware: adminWithStore,
			role:       pkg.RoleEditor,
			code:       http.StatusUnauthorized,
			desc:       "Reader admin, have write",
		},
		{
			middleware: adminWithStore,
			role:       pkg.RoleAdmin,
			code:       http.StatusOK,
			desc:       "Reader admin, have admin",
		},
		{
			middleware: adminWithoutSub,
			role:       pkg.RoleViewer,
			code:       http.StatusUnauthorized,
			desc:       "Require admin without sub, have read",
		},
		{
			middleware: adminWithoutSub,
			role:       pkg.RoleEditor,
			code:       http.StatusUnauthorized,
			desc:       "Reader admin without sub, have write",
		},
		{
			middleware: adminWithoutSub,
			role:       pkg.RoleAdmin,
			code:       http.StatusOK,
			desc:       "Reader admin without sub, have admin",
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
			session.IsNew = false
			if err != nil {
				t.Fatal(err)
			}

			ordId := "orgId"
			userInfos := pkg.UserInfo{
				Id: "aaa",
				Roles: map[string]pkg.RoleKind{
					ordId: test.role,
				},
			}

			data, err := json.Marshal(userInfos)
			if err != nil {
				t.Fatal(err)
			}

			session.Values["role"] = data
			session.Values["orgId"] = ordId

			config := pkg.NewDefaultConfig()
			middleware := test.middleware(config, cookie, &opt)

			ctx := context.WithValue(request.Context(), sessionKey, session)
			middleware(handler).ServeHTTP(recorder, request.WithContext(ctx))

			if recorder.Code != test.code {
				t.Fatalf("Wanted '%d' got '%d'", test.code, recorder.Code)
			}
		})
	}
}

func TestAccessMiddlewareBadRequestOnMissingSession(t *testing.T) {
	opt := sessions.Options{}
	cookie := sessions.NewCookieStore([]byte("key"))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	config := pkg.NewDefaultConfig()
	store := pkg.NewMultiOrgInMemoryStore()
	for i, middleware := range []func(http.Handler) http.Handler{
		RequireRead(cookie, &opt),
		RequireWrite(store, config, cookie, &opt),
		RequireAdmin(store, config, cookie, &opt),
	} {
		t.Run(fmt.Sprintf("Test: #%d", i), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "/endpoint", nil)
			session, err := cookie.Get(request, AuthSession)
			testutils.AssertNil(t, err)
			session.IsNew = false
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

func TestRequireUserId(t *testing.T) {
	req := httptest.NewRequest("GET", "/endpoint", nil)
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	store := sessions.NewCookieStore([]byte("top-secret"))
	session, err := store.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	ctx := context.WithValue(req.Context(), sessionKey, session)

	withMiddleware := RequireUserId(store)(http.HandlerFunc(handler))
	t.Run("Test missing user id", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		withMiddleware.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusBadRequest)
	})

	t.Run("Test with userId", func(t *testing.T) {
		session.Values["userId"] = "0000-0000"
		recorder := httptest.NewRecorder()
		withMiddleware.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
	})
}

func TestValidateUserInfo(t *testing.T) {
	req := httptest.NewRequest("GET", "/endpoint", nil)
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	store := sessions.NewCookieStore([]byte("top-secret"))
	session, err := store.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	withMiddleware := ValidateUserInfo(store)(http.HandlerFunc(handler))
	ctx := context.WithValue(req.Context(), sessionKey, session)

	t.Run("Test missing role", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		withMiddleware.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusBadRequest)
	})

	t.Run("Test role not byte", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		session.Values["role"] = "some string"
		withMiddleware.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusBadRequest)
	})

	t.Run("Test role not JSON", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		session.Values["role"] = []byte("not JSON")
		withMiddleware.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusBadRequest)
	})

	t.Run("Test role JSON", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		session.Values["role"] = []byte("{}")
		withMiddleware.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
	})
}

func TestRequireWriteSubscription(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	store.Organizations = []pkg.Organization{{Id: "org1"}, {Id: "org2"}, {Id: "org3"}}
	store.Subscriptions = map[string]pkg.Subscription{
		"org1": {Id: "sub1", MaxScores: 10, Expires: time.Now().Add(20 * time.Minute)},
		"org2": {Id: "sub2", MaxScores: 10, Expires: time.Now().Add(-20 * time.Minute)},
	}

	configNotRequire := pkg.NewDefaultConfig()
	configRequire := pkg.NewDefaultConfig()
	configRequire.RequireSubscription = true

	cookieStore := sessions.NewCookieStore([]byte("top-secret"))

	req := httptest.NewRequest("GET", "/what", nil)
	session, err := cookieStore.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	ctx := context.WithValue(context.Background(), sessionKey, session)

	clearSessionVals := func() {
		for k := range session.Values {
			delete(session.Values, k)
		}
	}

	t.Run("config-not-require", func(t *testing.T) {
		defer clearSessionVals()
		h := RequireWriteSubscription(store, configNotRequire)(handler)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusOK)
	})

	t.Run("config-require-missing-value", func(t *testing.T) {
		defer clearSessionVals()
		h := RequireWriteSubscription(store, configRequire)(handler)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusForbidden)
	})

	t.Run("config-org-without-sub-should-equal-free-tier", func(t *testing.T) {
		defer clearSessionVals()
		h := RequireWriteSubscription(store, configRequire)(handler)
		session.Values["orgId"] = "org3"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusOK)
	})

	t.Run("config-expired-subscription-forbidden", func(t *testing.T) {
		defer clearSessionVals()
		h := RequireWriteSubscription(store, configRequire)(handler)
		session.Values["orgId"] = "org2"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusForbidden)
	})

	t.Run("config-org-with-subscription-ok", func(t *testing.T) {
		defer clearSessionVals()
		h := RequireWriteSubscription(store, configRequire)(handler)
		session.Values["orgId"] = "org1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusOK)
	})

	t.Run("config-require-valid", func(t *testing.T) {
		defer clearSessionVals()
		h := RequireWriteSubscription(store, configRequire)(handler)
		session.Values[SubscriptionWriteAllowed] = true
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusOK)
	})
}

func TestNewSessionOnChangedSignKey(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		session := MustGetSession(r)
		if err := session.Save(r, w); err != nil {
			t.Fatal(err)
		}
	}

	cookieStore := sessions.NewCookieStore([]byte("sign-key-1"))
	wrappedHandler := RequireSession(cookieStore, AuthSession, &sessions.Options{})(http.HandlerFunc(handler))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/endpoint", nil)
	wrappedHandler.ServeHTTP(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusOK)
	cookies := rec.Result().Cookies()
	testutils.AssertEqual(t, len(cookies), 1)

	// Server has rotated the sign key
	cookieStore = sessions.NewCookieStore([]byte("sign-key-2"))
	wrappedHandler = RequireSession(cookieStore, AuthSession, &sessions.Options{})(http.HandlerFunc(handler))

	req = httptest.NewRequest("GET", "/endpoint", nil)
	req.AddCookie(cookies[0])

	rec = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusOK)
}

func TestNoErrorOnFailedSubscriptionSessionSave(t *testing.T) {
	store := brokenSessionStore{}
	req := httptest.NewRequest("GET", "/subscription", nil)
	session, err := store.Get(req, AuthSession)
	testutils.AssertNil(t, err)
	rec := httptest.NewRecorder()
	trySaveSession(session, req, rec)
}
