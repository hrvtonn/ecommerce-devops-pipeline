package httpapi

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// securityHeaders sets the same protective headers helmet applied in the
// Node.js version.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("X-DNS-Prefetch-Control", "off")
		h.Set("X-XSS-Protection", "0")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		// The Swagger UI loads its assets from a CDN, so it gets no CSP.
		if !strings.HasPrefix(r.URL.Path, "/api-docs") {
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}
		next.ServeHTTP(w, r)
	})
}

// cors allows requests from any origin and answers preflight requests.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,PUT,PATCH,POST,DELETE")
			if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverer turns a panic into a 500 response instead of dropping the connection.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				writeError(w, r, fmt.Errorf("panic: %v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// rateLimiter is a fixed-window, per-client-IP limiter that sends the
// RateLimit-* headers (IETF draft 6).
type rateLimiter struct {
	window    time.Duration
	max       int
	skip      map[string]bool
	mu        sync.Mutex
	clients   map[string]*clientWindow
	nextSweep time.Time
}

type clientWindow struct {
	count int
	reset time.Time
}

// newRateLimiter creates a limiter; requests to the paths in skip (health
// checks and Prometheus scrapes) are never limited.
func newRateLimiter(window time.Duration, max int, skip ...string) *rateLimiter {
	rl := &rateLimiter{window: window, max: max, skip: map[string]bool{}, clients: map[string]*clientWindow{}}
	for _, path := range skip {
		rl.skip[path] = true
	}
	return rl
}

func (rl *rateLimiter) hit(key string, now time.Time) (count int, reset time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Drop expired windows periodically so the map does not grow forever.
	if now.After(rl.nextSweep) {
		for k, c := range rl.clients {
			if now.After(c.reset) {
				delete(rl.clients, k)
			}
		}
		rl.nextSweep = now.Add(rl.window)
	}

	c, ok := rl.clients[key]
	if !ok || now.After(c.reset) {
		c = &clientWindow{reset: now.Add(rl.window)}
		rl.clients[key] = c
	}
	c.count++
	return c.count, c.reset
}

func (rl *rateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl.skip[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		now := time.Now()
		count, reset := rl.hit(ip, now)
		resetSeconds := strconv.Itoa(int(reset.Sub(now).Round(time.Second).Seconds()))

		h := w.Header()
		h.Set("RateLimit-Limit", strconv.Itoa(rl.max))
		h.Set("RateLimit-Remaining", strconv.Itoa(max(rl.max-count, 0)))
		h.Set("RateLimit-Reset", resetSeconds)

		if count > rl.max {
			slog.Warn("Rate limit exceeded", "ip", ip, "path", r.URL.Path)
			h.Set("Retry-After", resetSeconds)
			writeJSON(w, http.StatusTooManyRequests, errorBody{
				Status:  "error",
				Code:    "RATE_LIMIT_EXCEEDED",
				Message: "Too many requests, please try again later",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
