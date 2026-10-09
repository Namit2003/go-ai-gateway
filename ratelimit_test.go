package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func newTestLimiter(t *testing.T, limit rate.Limit, burst int) *IPRateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return NewIPRateLimiter(ctx, limit, burst)
}

func TestIPRateLimiterAllow(t *testing.T) {
	l := newTestLimiter(t, 0.001, 2)

	if !l.Allow("1.1.1.1") || !l.Allow("1.1.1.1") {
		t.Fatal("first 2 requests (the burst) should be allowed")
	}
	if l.Allow("1.1.1.1") {
		t.Error("3rd request should be rejected")
	}
	if !l.Allow("2.2.2.2") {
		t.Error("a different IP should have its own limit")
	}
}

func TestIPRateLimiterCleanup(t *testing.T) {
	l := newTestLimiter(t, 1, 1)
	l.Allow("1.1.1.1")

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

	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.RemoteAddr = "1.1.1.1:5000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, want)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = "1.1.1.1:6000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("same IP on a new port: status = %d, want 429", rec.Code)
	}

	req.RemoteAddr = "2.2.2.2:5000"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("different IP: status = %d, want 200", rec.Code)
	}
}
