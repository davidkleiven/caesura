package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
