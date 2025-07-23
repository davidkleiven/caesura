package api

import (
	"net/http/httptest"
	"testing"
)

func TestErrorOnUnknownRoute(t *testing.T) {
	transport := NewMockTransport()
	r := httptest.NewRequest("GET", "/route", nil)
	if _, err := transport.RoundTrip(r); err == nil {
		t.Fatalf("Wanted a non-nil error got nil")
	}
}
