package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestLimiter(clock *time.Time) *AuthLimiter {
	l := NewAuthLimiter()
	l.now = func() time.Time { return *clock }
	return l
}

func loginFrom(handler http.Handler, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	if ip != "" {
		req.Header.Set("CF-Connecting-IP", ip)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestLoginLockoutAfterFailures(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	s := &Server{
		JWTSecret:   "0123456789abcdef0123456789abcdef",
		CORSOrigin:  "http://localhost:3000",
		AuthLimiter: newTestLimiter(&clock),
	}
	handler := s.Routes()

	for i := 0; i < DefaultAuthMaxFailures; i++ {
		if rec := loginFrom(handler, "203.0.113.7"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, rec.Code)
		}
	}

	rec := loginFrom(handler, "203.0.113.7")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after %d failures, got %d", DefaultAuthMaxFailures, rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429")
	}

	// Lockout applies to the register endpoint too (same client).
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{}`))
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on register during lockout, got %d", rec.Code)
	}

	// A different client is unaffected.
	if rec := loginFrom(handler, "198.51.100.9"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected other client to get 401, got %d", rec.Code)
	}

	// Lockout expires.
	clock = clock.Add(DefaultAuthLockout + time.Second)
	if rec := loginFrom(handler, "203.0.113.7"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after lockout expiry, got %d", rec.Code)
	}
}

func TestAuthRateLimitPerWindow(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := newTestLimiter(&clock)
	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < DefaultAuthMaxAttempts; i++ {
		if rec := loginFrom(handler, "203.0.113.7"); rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: expected 200, got %d", i+1, rec.Code)
		}
	}
	if rec := loginFrom(handler, "203.0.113.7"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once window limit exceeded, got %d", rec.Code)
	}

	clock = clock.Add(DefaultAuthWindow)
	if rec := loginFrom(handler, "203.0.113.7"); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 in new window, got %d", rec.Code)
	}
}

func TestSuccessResetsFailures(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	l := newTestLimiter(&clock)
	status := http.StatusUnauthorized
	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))

	for i := 0; i < DefaultAuthMaxFailures-1; i++ {
		loginFrom(handler, "203.0.113.7")
	}
	status = http.StatusOK
	loginFrom(handler, "203.0.113.7")

	status = http.StatusUnauthorized
	clock = clock.Add(DefaultAuthWindow) // avoid the per-window cap
	if rec := loginFrom(handler, "203.0.113.7"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected failures to reset after success, got %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:4321"
	if got := clientIP(req); got != "10.0.0.5" {
		t.Fatalf("expected RemoteAddr host, got %q", got)
	}
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	if got := clientIP(req); got != "203.0.113.7" {
		t.Fatalf("expected CF-Connecting-IP, got %q", got)
	}
	req.Header.Set("CF-Connecting-IP", "not-an-ip")
	if got := clientIP(req); got != "10.0.0.5" {
		t.Fatalf("expected fallback on invalid header, got %q", got)
	}
}
