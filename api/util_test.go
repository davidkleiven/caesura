package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/sessions"
)

func TestDefaultPort(t *testing.T) {
	origPort := os.Getenv("PORT")
	if origPort != "" {
		defer os.Setenv("PORT", origPort)
	}

	os.Unsetenv("PORT")
	if Port() != ":8080" {
		t.Error("Expected default port to be :8080")
	}
}

func TestPortSetViaEnvironment(t *testing.T) {
	origPort := os.Getenv("PORT")
	if origPort != "" {
		defer os.Setenv("PORT", origPort)
	}

	os.Setenv("PORT", "1234")
	if Port() != ":1234" {
		t.Error("Expected port to be set to :1234")
	}
}

func TestIncludeError(t *testing.T) {
	recorder := httptest.NewRecorder()
	includeError(recorder, http.StatusInternalServerError, "Test error", fmt.Errorf("This is a test error"))
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, recorder.Code)
	}

	want := "Test error: This is a test error\n"
	if recorder.Body.String() != want {
		t.Errorf("Expected body to be '%s', got '%s'", want, recorder.Body.String())
	}
}

func TestIncludeErrorOnNoError(t *testing.T) {
	recorder := httptest.NewRecorder()
	includeError(recorder, http.StatusInternalServerError, "Test error", nil)
	if recorder.Code == http.StatusInternalServerError {
		t.Errorf("Code should not be set by error function. Got %d", recorder.Code)
	}

	if recorder.Body.String() != "" {
		t.Errorf("Expected body to be empty, got '%s'", recorder.Body.String())
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
			t.Errorf("Expected panic, but function did not panic")
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
			t.Errorf("Expected panic, but function did not panic")
		}
	}()

	MustGetOrgId(session)
}
