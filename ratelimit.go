package main

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	cleanupInterval = time.Minute
	idleTTL         = 5 * time.Minute
)

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu      sync.Mutex
	clients map[string]*client
	limit   rate.Limit
	burst   int
}

func NewIPRateLimiter(ctx context.Context, limit rate.Limit, burst int) *IPRateLimiter {
	l := &IPRateLimiter{clients: make(map[string]*client), limit: limit, burst: burst}
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.cleanup(idleTTL)
			}
		}
	}()
	return l
}

func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.clients[ip]
	if !ok {
		c = &client{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.clients[ip] = c
	}
	c.lastSeen = time.Now()
	return c.limiter.Allow()
}

func (l *IPRateLimiter) cleanup(maxIdle time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, c := range l.clients {
		if time.Since(c.lastSeen) > maxIdle {
			delete(l.clients, ip)
		}
	}
}

func rateLimit(l *IPRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !l.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "rate_limit_error", "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
