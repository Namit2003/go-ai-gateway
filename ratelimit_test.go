package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func newTestLimiter(t *testing.T, limit rate.Limit, burst int) *ClientRateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return NewClientRateLimiter(ctx, limit, burst)
}

func TestClientRateLimiterAllow(t *testing.T) {
	l := newTestLimiter(t, 0.001, 2)

	if !l.Allow("alice") || !l.Allow("alice") {
		t.Fatal("first 2 requests (the burst) should be allowed")
	}
	if l.Allow("alice") {
		t.Error("3rd request should be rejected")
	}
	if !l.Allow("bob") {
		t.Error("a different client should have its own limit")
	}
}

func TestClientRateLimiterCleanup(t *testing.T) {
	l := newTestLimiter(t, 1, 1)
	l.Allow("alice")

	l.cleanup(idleTTL)
	if len(l.clients) != 1 {
		t.Errorf("recently seen client was removed, have %d clients", len(l.clients))
	}

	l.cleanup(0)
	if len(l.clients) != 0 {
		t.Errorf("idle client was not removed, have %d clients", len(l.clients))
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	l := newTestLimiter(t, 0.001, 1)
	h := rateLimit(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(client, remoteAddr string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.RemoteAddr = remoteAddr
		req = req.WithContext(context.WithValue(req.Context(), clientKey{}, client))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := send("alice", "1.1.1.1:5000"); got != http.StatusOK {
		t.Errorf("first request: status = %d, want 200", got)
	}
	if got := send("alice", "1.1.1.1:5000"); got != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want 429", got)
	}
	if got := send("alice", "9.9.9.9:5000"); got != http.StatusTooManyRequests {
		t.Errorf("same client from another IP: status = %d, want 429", got)
	}
	if got := send("bob", "1.1.1.1:5000"); got != http.StatusOK {
		t.Errorf("different client on the same IP: status = %d, want 200", got)
	}
}
