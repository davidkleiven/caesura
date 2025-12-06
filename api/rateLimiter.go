package api

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

type Observation struct {
	Num        float64
	LastUpdate time.Time
}

func smoothFactor(x float64) float64 {
	if x > 1.0 {
		return 0.0
	}
	return 1.0 - x
}

func getIp(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func normalizedTime(lastUpdate time.Time, decayRate time.Duration) float64 {
	return float64(time.Since(lastUpdate)) / float64(decayRate)
}

type RateLimiter struct {
	MaxNumRequests float64
	DecayRate      time.Duration
	RequestCount   map[string]Observation
	mu             sync.Mutex
}

func (rl *RateLimiter) Allowed(ipAddr string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	current, ok := rl.RequestCount[ipAddr]
	if ok {
		factor := normalizedTime(current.LastUpdate, rl.DecayRate)
		current.Num = 1.0 + smoothFactor(factor)*current.Num
		current.LastUpdate = time.Now()
		rl.RequestCount[ipAddr] = current
		return current.Num < rl.MaxNumRequests
	}

	rl.RequestCount[ipAddr] = Observation{Num: 1.0, LastUpdate: time.Now()}
	return true
}

func (rl *RateLimiter) Cleanup() {
	var (
		maxIpAddr string
		maxCount  float64
	)
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for ipAddr, obs := range rl.RequestCount {
		if obs.Num > maxCount {
			maxCount = obs.Num
			maxIpAddr = ipAddr
		}
		if normalizedTime(obs.LastUpdate, rl.DecayRate) >= 1.0 {
			delete(rl.RequestCount, ipAddr)
		}
	}

	slog.Info("Current maximum request count in rate limiter", "ip", maxIpAddr, "count", maxCount, "fillRate", maxCount/rl.MaxNumRequests)
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ipAddr := getIp(r)
		if !rl.Allowed(ipAddr) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.DecayRate.Seconds())))
			fmt.Fprintf(w, "Too many requests, retry later")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func NewRateLimiter(Max float64, decayRate time.Duration) *RateLimiter {
	return &RateLimiter{
		MaxNumRequests: Max,
		DecayRate:      decayRate,
		RequestCount:   make(map[string]Observation),
	}
}
