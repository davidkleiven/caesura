package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/testutils"
)

func TestGetIp(t *testing.T) {
	req := httptest.NewRequest("GET", "/whatever", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	testutils.AssertEqual(t, getIp(req), "127.0.0.1")
}

func TestGetIpInvalidIp(t *testing.T) {
	req := httptest.NewRequest("GET", "/whatever", nil)
	req.RemoteAddr = "not-an-ip-address"
	testutils.AssertEqual(t, getIp(req), "not-an-ip-address")
}

func TestRateLimiterAllowed(t *testing.T) {
	limiter := NewRateLimiter(1.0, time.Minute)
	ipAddr := "127.0.0.1"
	testutils.AssertEqual(t, limiter.Allowed(ipAddr), true)
	testutils.AssertEqual(t, len(limiter.RequestCount), 1)
	testutils.AssertEqual(t, limiter.Allowed(ipAddr), false)
	value := limiter.RequestCount["127.0.0.1"].Num
	if value <= 1.0 || value >= 2.0 {
		t.Fatalf("Value should be between 1.0 and 2.0. Got %.2f", value)
	}
}

func TestCleanUp(t *testing.T) {
	limiter := NewRateLimiter(1.0, time.Minute)

	// Should be deleted
	limiter.RequestCount["a"] = Observation{Num: 1, LastUpdate: time.Now().Add(-2 * time.Minute)}

	// Should not be deleted
	limiter.RequestCount["b"] = Observation{Num: 5, LastUpdate: time.Now()}

	limiter.Cleanup()
	_, okA := limiter.RequestCount["a"]
	_, okB := limiter.RequestCount["b"]
	testutils.AssertEqual(t, okA, false)
	testutils.AssertEqual(t, okB, true)
}

func TestRateLimiterMiddleware(t *testing.T) {
	called := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
	}

	limiter := NewRateLimiter(1.0, time.Minute)
	wrappedHandler := limiter.Middleware(http.HandlerFunc(handler))

	limiter.RequestCount["127.0.0.1"] = Observation{LastUpdate: time.Now(), Num: 5.0}

	req := httptest.NewRequest("GET", "/whatever", nil)
	rec := httptest.NewRecorder()

	req.RemoteAddr = "127.0.0.1:8080"
	wrappedHandler.ServeHTTP(rec, req)
	testutils.AssertEqual(t, len(limiter.RequestCount), 1)
	testutils.AssertEqual(t, rec.Code, http.StatusTooManyRequests)
	testutils.AssertEqual(t, rec.Header().Get("Retry-After"), "60")
	testutils.AssertEqual(t, called, false)

	req.RemoteAddr = "127.0.0.2:7876"
	rec = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)
	testutils.AssertEqual(t, rec.Code, http.StatusOK)
	testutils.AssertEqual(t, called, true)
}
