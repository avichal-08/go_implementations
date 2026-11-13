package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testHandler(t *testing.T, cfg config) (http.Handler, *atomic.Bool) {
	t.Helper()
	if cfg.maxInFlight == 0 {
		cfg.maxInFlight = 10
	}
	if cfg.requestTimeout == 0 {
		cfg.requestTimeout = 5 * time.Second
	}
	var ready atomic.Bool
	ready.Store(true)
	return newHandler(cfg, &ready, slog.New(slog.NewTextHandler(io.Discard, nil))), &ready
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestHello(t *testing.T) {
	h, _ := testHandler(t, config{})
	rec := do(h, "GET", "/hello", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header not set")
	}
}

func TestRequestIDIsReused(t *testing.T) {
	h, _ := testHandler(t, config{})
	req := httptest.NewRequest("GET", "/hello", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "abc-123" {
		t.Errorf("X-Request-ID = %q, want abc-123", got)
	}
	if !strings.Contains(rec.Body.String(), "abc-123") {
		t.Errorf("request ID not propagated to handler: %s", rec.Body)
	}
}

func TestPanicIsRecovered(t *testing.T) {
	h, _ := testHandler(t, config{})
	rec := do(h, "GET", "/panic", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	// server must keep working after a panic
	if rec := do(h, "GET", "/hello", ""); rec.Code != http.StatusOK {
		t.Fatalf("status after panic = %d, want 200", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h, _ := testHandler(t, config{})
	if rec := do(h, "POST", "/hello", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestEcho(t *testing.T) {
	h, _ := testHandler(t, config{})
	tests := []struct {
		name string
		body string
		want int
	}{
		{"valid", `{"message":"hi"}`, http.StatusOK},
		{"malformed", `{"message":`, http.StatusBadRequest},
		{"unknown field", `{"nope":1}`, http.StatusBadRequest},
		{"too large", `{"message":"` + strings.Repeat("a", maxBodyBytes) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(h, "POST", "/echo", tc.body); rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestReadyzFlipsOnShutdown(t *testing.T) {
	h, ready := testHandler(t, config{})
	if rec := do(h, "GET", "/readyz", ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ready.Store(false)
	if rec := do(h, "GET", "/readyz", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if rec := do(h, "GET", "/healthz", ""); rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", rec.Code)
	}
}

func TestRequestTimeout(t *testing.T) {
	h, _ := testHandler(t, config{requestTimeout: 50 * time.Millisecond})
	start := time.Now()
	rec := do(h, "GET", "/slow?d=10s", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v, handler did not stop at the deadline", elapsed)
	}
}

func TestLimitInFlightShedsLoad(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	h := limitInFlight(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	}))

	done := make(chan int)
	go func() { done <- do(h, "GET", "/", "").Code }()
	<-started // the only slot is now taken

	rec := do(h, "GET", "/", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header not set")
	}

	close(release)
	if code := <-done; code != http.StatusOK {
		t.Errorf("first request status = %d, want 200", code)
	}

	// slot is free again
	h = limitInFlight(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if rec := do(h, "GET", "/", ""); rec.Code != http.StatusOK {
		t.Errorf("status after release = %d, want 200", rec.Code)
	}
}
