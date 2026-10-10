package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQuotaTrackerAllow(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
	q := NewQuotaTracker(2)
	q.now = func() time.Time { return now }

	if !q.Allow("alice") || !q.Allow("alice") {
		t.Fatal("first 2 requests should be allowed")
	}
	if q.Allow("alice") {
		t.Error("3rd request should be rejected")
	}
	if !q.Allow("bob") {
		t.Error("a different client should have its own quota")
	}

	now = now.Add(10 * time.Hour)
	if !q.Allow("alice") {
		t.Error("after UTC midnight the quota should reset")
	}
}

func TestQuotaTrackerSameDayDoesNotReset(t *testing.T) {
	now := time.Date(2026, 10, 9, 0, 0, 1, 0, time.UTC)
	q := NewQuotaTracker(1)
	q.now = func() time.Time { return now }

	q.Allow("alice")
	now = now.Add(23 * time.Hour)
	if q.Allow("alice") {
		t.Error("still the same UTC day, should be rejected")
	}
}

func TestQuotaMiddleware(t *testing.T) {
	q := NewQuotaTracker(1)
	h := quota(q, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(client string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req = req.WithContext(context.WithValue(req.Context(), clientKey{}, client))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := send("alice"); rec.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want 200", rec.Code)
	}
	rec := send("alice")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want 429", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, "insufficient_quota") {
		t.Errorf("body = %s, want type insufficient_quota", got)
	}
	if rec := send("bob"); rec.Code != http.StatusOK {
		t.Errorf("different client: status = %d, want 200", rec.Code)
	}
}
