package main

import (
	"context"
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

type ClientRateLimiter struct {
	mu      sync.Mutex
	clients map[string]*client
	limit   rate.Limit
	burst   int
}

func NewClientRateLimiter(ctx context.Context, limit rate.Limit, burst int) *ClientRateLimiter {
	l := &ClientRateLimiter{clients: make(map[string]*client), limit: limit, burst: burst}
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

func (l *ClientRateLimiter) Allow(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.clients[name]
	if !ok {
		c = &client{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.clients[name] = c
	}
	c.lastSeen = time.Now()
	return c.limiter.Allow()
}

func (l *ClientRateLimiter) cleanup(maxIdle time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for name, c := range l.clients {
		if time.Since(c.lastSeen) > maxIdle {
			delete(l.clients, name)
		}
	}
}

func rateLimit(l *ClientRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, _ := r.Context().Value(clientKey{}).(string)
		if !l.Allow(name) {
			writeError(w, http.StatusTooManyRequests, "rate_limit_error", "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
