package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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
