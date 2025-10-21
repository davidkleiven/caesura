package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
)

func TestDefaultPort(t *testing.T) {
	origPort := os.Getenv("PORT")
	if origPort != "" {
		defer os.Setenv("PORT", origPort)
	}

	os.Unsetenv("PORT")
	if Port() != ":8080" {
		t.Fatal("Expected default port to be :8080")
	}
}

func TestPortSetViaEnvironment(t *testing.T) {
	origPort := os.Getenv("PORT")
	if origPort != "" {
		defer os.Setenv("PORT", origPort)
	}

	os.Setenv("PORT", "1234")
	if Port() != ":1234" {
		t.Fatal("Expected port to be set to :1234")
	}
}

func TestIncludeError(t *testing.T) {
	recorder := httptest.NewRecorder()
	includeError(recorder, http.StatusInternalServerError, "Test error", fmt.Errorf("This is a test error"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code %d, got %d", http.StatusInternalServerError, recorder.Code)
	}

	want := "Test error: This is a test error\n"
	if recorder.Body.String() != want {
		t.Fatalf("Expected body to be '%s', got '%s'", want, recorder.Body.String())
	}
}

func TestIncludeErrorOnNoError(t *testing.T) {
	recorder := httptest.NewRecorder()
	includeError(recorder, http.StatusInternalServerError, "Test error", nil)
	if recorder.Code == http.StatusInternalServerError {
		t.Fatalf("Code should not be set by error function. Got %d", recorder.Code)
	}

	if recorder.Body.String() != "" {
		t.Fatalf("Expected body to be empty, got '%s'", recorder.Body.String())
	}
}

func TestCodeAndMessage(t *testing.T) {
	for _, test := range []struct {
		err      error
		code     int
		wantMsg  string
		wantCode int
	}{
		{
			code:     200,
			wantCode: http.StatusInternalServerError,
		},
		{
			code:     401,
			wantCode: 401,
		},
		{
			err:      errors.New("error"),
			code:     401,
			wantCode: 401,
			wantMsg:  "error",
		},
	} {
		name := fmt.Sprintf("Code: %d err: %v", test.code, test.err)
		t.Run(name, func(t *testing.T) {
			msg, code := CodeAndMessage(test.err, test.code)

			if msg != test.wantMsg {
				t.Fatalf("Wanted '%s' got '%s'", test.wantMsg, msg)
			}

			if code != test.wantCode {
				t.Fatalf("Wanted '%d' got '%d'", test.wantCode, code)
			}
		})

	}
}

func TestPanicOnMissingSession(t *testing.T) {
	request := httptest.NewRequest("GET", "/login", nil)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic, but function did not panic")
		}
	}()
	MustGetSession(request)
}

func TestPanicOnWrongOrgIdType(t *testing.T) {
	store := sessions.NewCookieStore([]byte("whatever key"))

	request := httptest.NewRequest("GET", "/login", nil)
	session, err := store.Get(request, "session")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic, but function did not panic")
		}
	}()

	MustGetOrgId(session)
}

func TestOrganizationIds(t *testing.T) {
	info := pkg.UserInfo{
		Roles: map[string]pkg.RoleKind{
			"org1": pkg.RoleViewer,
			"org2": pkg.RoleEditor,
		},
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		roleData []byte
		wantIds  []string
		desc     string
	}{
		{
			roleData: nil,
			wantIds:  []string{},
			desc:     "no role data",
		},
		{
			roleData: data,
			wantIds:  []string{"org1", "org2"},
			desc:     "valid json",
		},
		{
			roleData: []byte("not a json"),
			wantIds:  []string{},
			desc:     "invalid json",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			session := sessions.Session{}
			if test.roleData != nil {
				session.Values = map[any]any{"role": test.roleData}
			}

			ids := organizationIds(&session)
			slices.Sort(ids)
			if slices.Compare(ids, test.wantIds) != 0 {
				t.Fatalf("Wanted '%v' got '%v'", test.wantIds, ids)
			}
		})
	}
}

func TestMustGetUserId(t *testing.T) {
	session := sessions.Session{
		Values: map[any]any{"userId": "user"},
	}

	extractedId := MustGetUserId(&session)
	if extractedId != "user" {
		t.Fatalf("Wanted '%s' got 'user'", extractedId)
	}
}

func TestMustGetUserIdPanicWhenMissing(t *testing.T) {
	session := sessions.Session{}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Did not panic")
		}
	}()

	MustGetUserId(&session)
}

func TestOrganizationIdFromInviteToken(t *testing.T) {
	orgId := "org"
	signSecret := "secret-value"
	currentTime := time.Now()

	for _, test := range []struct {
		expireAt       *jwt.NumericDate
		tokenChange    string
		wantErr        bool
		skipForSession bool
		expectedId     string
		desc           string
	}{
		{
			desc:       "Valid token",
			expectedId: orgId,
		},
		{
			desc:        "Edited token",
			tokenChange: "a",
			wantErr:     true,
		},
		{
			desc:     "Expired token",
			expireAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			wantErr:  true,
		},
		{
			desc:           "Missing inviteToken",
			skipForSession: true,
		},
	} {
		claims := InviteClaim{
			OrgId: orgId,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(currentTime.Add(48 * time.Hour)),
			},
		}

		if test.expireAt != nil {
			claims.ExpiresAt = test.expireAt
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		signedToken, err := token.SignedString([]byte(signSecret))

		signedToken += test.tokenChange

		if err != nil {
			t.Fatal(err)
		}

		session := sessions.Session{}

		if !test.skipForSession {
			session.Values = map[any]any{"invite-token": signedToken}
		}

		extractedOrgId, err := orgIdFromInviteToken(&session, signSecret)

		if err == nil && test.wantErr {
			t.Fatalf("Did not expect an error got %v", err)
		}

		if extractedOrgId != test.expectedId {
			t.Fatalf("Wanted '%s' got '%s", extractedOrgId, test.expectedId)
		}
	}
}

func TestEmptyOrIdOnInvalidClaim(t *testing.T) {
	claims := jwt.MapClaims{"orgId": "aaa-bbb"}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secretKey := "top-secret"
	signedToken, err := token.SignedString([]byte(secretKey))
	if err != nil {
		t.Fatal(err)
	}

	session := sessions.Session{
		Values: map[any]any{"invite-token": signedToken},
	}

	orgId, err := orgIdFromInviteToken(&session, secretKey)
	if orgId != "" {
		t.Fatalf("Wanted empty organization id got %s", orgId)
	}

	if err == nil {
		t.Fatal("Expectet non-nil error")
	}
}

func TestMustGetUserInfo(t *testing.T) {
	session := sessions.Session{
		Values: make(map[any]any),
	}

	t.Run("No role", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Did not panic")
			}
		}()
		MustGetUserInfo(&session)
	})

	t.Run("Not byte", func(t *testing.T) {
		session.Values["role"] = "some string"
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Did not panic")
			}
		}()
		MustGetUserInfo(&session)
	})

	t.Run("Not JSON", func(t *testing.T) {
		session.Values["role"] = []byte("not json")
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Did not panic")
			}
		}()
		MustGetUserInfo(&session)
	})

	t.Run("Valid JSON", func(t *testing.T) {
		session.Values["role"] = []byte("{}")
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Did not panic")
			}
		}()
		MustGetUserInfo(&session)
	})
}

func TestValidEmail(t *testing.T) {
	for _, test := range []struct {
		email string
		want  bool
	}{
		{
			email: "john@example.com",
			want:  true,
		},
		{
			email: "john@example.c",
			want:  false,
		},
		{
			email: "",
			want:  false,
		},
		{
			email: "@example.com",
			want:  false,
		},
	} {
		testutils.AssertEqual(t, validEmail(test.email), test.want)
	}
}
