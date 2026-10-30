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
	q := NewQuotaTracker(100)
	q.now = func() time.Time { return now }

	if !q.Allow("alice") {
		t.Fatal("a client with no usage should be allowed")
	}
	q.Add("alice", 100)
	if q.Allow("alice") {
		t.Error("a client at the limit should be rejected")
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

	q.Add("alice", 1)
	now = now.Add(23 * time.Hour)
	if q.Allow("alice") {
		t.Error("still the same UTC day, should be rejected")
	}
}

func TestQuotaMiddleware(t *testing.T) {
	q := NewQuotaTracker(10)
	h := quota(q, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestInfoFrom(r.Context()).Tokens = 10
		w.WriteHeader(http.StatusOK)
	}))

	send := func(client string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx := context.WithValue(req.Context(), clientKey{}, client)
		req = req.WithContext(context.WithValue(ctx, requestInfoKey{}, &requestInfo{}))
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

func TestQuotaCountsResponseTokens(t *testing.T) {
	q := NewQuotaTracker(100)
	provider := &recordingProvider{resp: ChatCompletionResponse{Usage: &Usage{TotalTokens: 150}}}
	h := logRequests(quota(q, chatCompletionsHandler(provider)))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	req = req.WithContext(context.WithValue(req.Context(), clientKey{}, "alice"))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if q.Allow("alice") {
		t.Error("150 tokens used against a limit of 100, should be rejected")
	}
}

func TestQuotaCountsStreamTokens(t *testing.T) {
	q := NewQuotaTracker(100)
	chunks := []ChatCompletionChunk{
		{Choices: []ChunkChoice{{Delta: Delta{Content: "hi"}}}},
		{Usage: &Usage{TotalTokens: 150}},
	}
	h := logRequests(quota(q, chatCompletionsHandler(fakeProvider{chunks: chunks})))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	req = req.WithContext(context.WithValue(req.Context(), clientKey{}, "alice"))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if q.Allow("alice") {
		t.Error("150 streamed tokens used against a limit of 100, should be rejected")
	}
}
