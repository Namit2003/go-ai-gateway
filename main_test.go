package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"golang.org/x/time/rate"
)

type fakeProvider struct {
	resp      ChatCompletionResponse
	chunks    []ChatCompletionChunk
	streamErr error
	err       error
}

type fakeStream struct {
	chunks []ChatCompletionChunk
	err    error
}

func (s *fakeStream) Recv() (ChatCompletionChunk, error) {
	if len(s.chunks) == 0 {
		if s.err != nil {
			return ChatCompletionChunk{}, s.err
		}
		return ChatCompletionChunk{}, io.EOF
	}
	chunk := s.chunks[0]
	s.chunks = s.chunks[1:]
	return chunk, nil
}

func (s *fakeStream) Close() error { return nil }

func (f fakeProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	return f.resp, f.err
}

func (f fakeProvider) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &fakeStream{chunks: f.chunks, err: f.streamErr}, nil
}

func TestChatCompletionsHandler(t *testing.T) {
	validBody := `{"model":"m","messages":[{"role":"user","content":"hi"}]}`

	tests := []struct {
		name       string
		body       string
		provider   fakeProvider
		wantStatus int
		wantInBody string
	}{
		{"invalid JSON", "not json", fakeProvider{}, http.StatusBadRequest, "invalid JSON"},
		{"missing model", `{"messages":[{"role":"user","content":"hi"}]}`, fakeProvider{}, http.StatusBadRequest, "model is required"},
		{"empty messages", `{"model":"m","messages":[]}`, fakeProvider{}, http.StatusBadRequest, "messages is required"},
		{"upstream rate limited", validBody, fakeProvider{err: &UpstreamError{StatusCode: 429}}, http.StatusTooManyRequests, "rate_limit_error"},
		{"no route", validBody, fakeProvider{err: ErrNoRoute}, http.StatusBadRequest, "no route matches"},
		{"circuit open", validBody, fakeProvider{err: ErrCircuitOpen}, http.StatusServiceUnavailable, "temporarily unavailable"},
		{"upstream failure", validBody, fakeProvider{err: errors.New("boom")}, http.StatusBadGateway, "upstream_error"},
		{
			"success",
			validBody,
			fakeProvider{resp: ChatCompletionResponse{Model: "m", Choices: []Choice{{Index: 0, Message: Message{Role: "assistant", Content: "hello"}}}}},
			http.StatusOK,
			"hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			chatCompletionsHandler(tt.provider)(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantInBody) {
				t.Errorf("body = %q, want it to contain %q", rec.Body.String(), tt.wantInBody)
			}
		})
	}
}

func TestChatCompletionsHandlerStream(t *testing.T) {
	stop := "stop"
	chunks := []ChatCompletionChunk{
		{ID: "c1", Object: "chat.completion.chunk", Created: 1, Model: "m", Choices: []ChunkChoice{{Delta: Delta{ReasoningContent: "think"}}}},
		{ID: "c1", Object: "chat.completion.chunk", Created: 1, Model: "m", Choices: []ChunkChoice{{Delta: Delta{Content: "hi"}}}},
		{ID: "c1", Object: "chat.completion.chunk", Created: 1, Model: "m", Choices: []ChunkChoice{{FinishReason: &stop}}},
	}
	events := `data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"reasoning_content":"think"},"finish_reason":null}]}

data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}

data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`
	body := `{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	chatCompletionsHandler(fakeProvider{chunks: chunks})(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", got, "text/event-stream")
	}
	if rec.Body.String() != events {
		t.Errorf("body = %q, want %q", rec.Body.String(), events)
	}
}

func TestChatCompletionsHandlerStreamError(t *testing.T) {
	body := `{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`

	tests := []struct {
		name         string
		cancelled    bool
		wantLog      string
		wantNotInLog string
	}{
		{"client disconnected", true, "client disconnected", "stream failed"},
		{"upstream failure", false, "stream failed", "client disconnected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs strings.Builder
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			ctx, cancel := context.WithCancel(context.Background())
			if tt.cancelled {
				cancel()
			}
			defer cancel()

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)).WithContext(ctx)
			rec := httptest.NewRecorder()

			chatCompletionsHandler(fakeProvider{streamErr: errors.New("boom")})(rec, req)

			if !strings.Contains(logs.String(), tt.wantLog) {
				t.Errorf("log = %q, want it to contain %q", logs.String(), tt.wantLog)
			}
			if strings.Contains(logs.String(), tt.wantNotInLog) {
				t.Errorf("log = %q, want it not to contain %q", logs.String(), tt.wantNotInLog)
			}
			if strings.Contains(rec.Body.String(), "[DONE]") {
				t.Errorf("body = %q, want no [DONE] after a failed stream", rec.Body.String())
			}
		})
	}
}

func BenchmarkRequestChain(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	keys := map[string]string{hashKey("sk-good"): "alice"}
	provider := &recordingProvider{resp: ChatCompletionResponse{ID: "x"}}
	h := requireAPIKey(keys, rateLimit(NewClientRateLimiter(ctx, rate.Inf, 1), limitConcurrency(100, quota(NewQuotaTracker(1<<30), chatCompletionsHandler(provider)))))

	const body = `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-good")
	rec := httptest.NewRecorder()
	for b.Loop() {
		req.Body = io.NopCloser(strings.NewReader(body))
		rec.Body.Reset()
		h.ServeHTTP(rec, req)
	}
}
