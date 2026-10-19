package handlers

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Default brute-force limits for the unauthenticated auth endpoints (/login, /register).
const (
	DefaultAuthMaxAttempts = 10               // requests per client per window
	DefaultAuthWindow      = time.Minute      // fixed window for MaxAttempts
	DefaultAuthMaxFailures = 5                // failed attempts before lockout
	DefaultAuthLockout     = 15 * time.Minute // lockout duration once MaxFailures is reached
)

// AuthLimiter enforces per-client request rate limits and a failure lockout.
// Clients are identified by the Cloudflare-supplied CF-Connecting-IP header
// (the backend is only reachable through the Cloudflare tunnel, which
// overwrites that header), falling back to the TCP peer address.
type AuthLimiter struct {
	MaxAttempts int
	Window      time.Duration
	MaxFailures int
	Lockout     time.Duration

	mu        sync.Mutex
	clients   map[string]*limiterEntry
	lastSweep time.Time
	now       func() time.Time
}

type limiterEntry struct {
	windowStart time.Time
	attempts    int
	failures    int
	lastFailure time.Time
	lockedUntil time.Time
}

func NewAuthLimiter() *AuthLimiter {
	return &AuthLimiter{
		MaxAttempts: DefaultAuthMaxAttempts,
		Window:      DefaultAuthWindow,
		MaxFailures: DefaultAuthMaxFailures,
		Lockout:     DefaultAuthLockout,
		clients:     map[string]*limiterEntry{},
		now:         time.Now,
	}
}

// clientIP returns the real client IP. Cloudflare sets CF-Connecting-IP on every
// proxied request and replaces any client-supplied value, so it cannot be spoofed
// from the public internet.
func clientIP(r *http.Request) string {
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); ip != nil {
		return ip.String()
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// allow records an attempt for key and reports whether it may proceed. When it
// may not, it returns how long the client should wait.
func (l *AuthLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	e, ok := l.clients[key]
	if !ok {
		e = &limiterEntry{windowStart: now}
		l.clients[key] = e
	}

	if now.Before(e.lockedUntil) {
		return false, e.lockedUntil.Sub(now)
	}
	if now.Sub(e.windowStart) >= l.Window {
		e.windowStart = now
		e.attempts = 0
	}
	if e.attempts >= l.MaxAttempts {
		return false, e.windowStart.Add(l.Window).Sub(now)
	}
	e.attempts++
	return true, 0
}

// record updates the failure count for key based on the response status.
// Successful responses clear failures; client errors count toward lockout.
func (l *AuthLimiter) record(key string, status int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.clients[key]
	if !ok {
		return
	}
	switch {
	case status >= 200 && status < 300:
		e.failures = 0
	case status >= 400 && status < 500 && status != http.StatusTooManyRequests:
		now := l.now()
		// Failures older than the lockout period no longer count.
		if now.Sub(e.lastFailure) >= l.Lockout {
			e.failures = 0
		}
		e.failures++
		e.lastFailure = now
		if e.failures >= l.MaxFailures {
			e.lockedUntil = now.Add(l.Lockout)
			e.failures = 0
			log.Printf("ratelimit: client %s locked out for %s after %d failed auth attempts", key, l.Lockout, l.MaxFailures)
		}
	}
}

// sweep drops idle entries so the map cannot grow without bound. Caller holds l.mu.
func (l *AuthLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.Window {
		return
	}
	l.lastSweep = now
	for k, e := range l.clients {
		if now.After(e.lockedUntil) &&
			now.Sub(e.windowStart) >= l.Window &&
			now.Sub(e.lastFailure) >= l.Lockout {
			delete(l.clients, k)
		}
	}
}

// Middleware wraps brute-force-sensitive handlers with rate limiting and lockout.
func (l *AuthLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		ok, retry := l.allow(key)
		if !ok {
			secs := int(retry.Round(time.Second) / time.Second)
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			writeErr(w, http.StatusTooManyRequests, "too many attempts; please try again later")
			return
		}
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		l.record(key, sw.status)
	})
}
