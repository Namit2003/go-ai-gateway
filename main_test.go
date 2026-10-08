package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeProvider struct {
	resp   ChatCompletionResponse
	stream string
	err    error
}

func (f fakeProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	return f.resp, f.err
}

func (f fakeProvider) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.stream)), f.err
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
	events := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"
	body := `{"model":"m","messages":[{"role":"user","content":"hi"}],"stream":true}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	chatCompletionsHandler(fakeProvider{stream: events})(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", got, "text/event-stream")
	}
	if rec.Body.String() != events {
		t.Errorf("body = %q, want %q", rec.Body.String(), events)
	}
}
