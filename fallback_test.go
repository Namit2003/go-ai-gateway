package main

import (
	"context"
	"errors"
	"testing"
)

type recordingProvider struct {
	resp     ChatCompletionResponse
	err      error
	gotModel string
	called   bool
}

func (p *recordingProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	p.called = true
	p.gotModel = req.Model
	return p.resp, p.err
}

func (p *recordingProvider) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
	p.called = true
	p.gotModel = req.Model
	if p.err != nil {
		return nil, p.err
	}
	return &fakeStream{}, nil
}

func TestFallbackProviderChatCompletion(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name         string
		ctx          context.Context
		primaryErr   error
		fallbackErr  error
		wantFallback bool
		wantErr      error
	}{
		{"primary succeeds", context.Background(), nil, nil, false, nil},
		{"primary 503", context.Background(), &UpstreamError{StatusCode: 503}, nil, true, nil},
		{"primary network error", context.Background(), errors.New("boom"), nil, true, nil},
		{"primary circuit open", context.Background(), ErrCircuitOpen, nil, true, nil},
		{"primary 401", context.Background(), &UpstreamError{StatusCode: 401}, nil, false, &UpstreamError{StatusCode: 401}},
		{"client cancelled", cancelled, errors.New("boom"), nil, false, errors.New("boom")},
		{"both fail", context.Background(), &UpstreamError{StatusCode: 503}, ErrCircuitOpen, true, ErrCircuitOpen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &recordingProvider{resp: ChatCompletionResponse{ID: "primary"}, err: tt.primaryErr}
			fallback := &recordingProvider{resp: ChatCompletionResponse{ID: "fallback"}, err: tt.fallbackErr}
			p := FallbackProvider{Primary: primary, Fallback: fallback, FallbackModel: "qwen"}

			resp, err := p.ChatCompletion(tt.ctx, ChatCompletionRequest{Model: "gpt-4o"})

			if fallback.called != tt.wantFallback {
				t.Errorf("fallback called = %v, want %v", fallback.called, tt.wantFallback)
			}
			if tt.wantFallback && fallback.gotModel != "qwen" {
				t.Errorf("fallback model = %q, want %q", fallback.gotModel, "qwen")
			}
			if primary.gotModel != "gpt-4o" {
				t.Errorf("primary model = %q, want %q", primary.gotModel, "gpt-4o")
			}
			if (err == nil) != (tt.wantErr == nil) || (err != nil && err.Error() != tt.wantErr.Error()) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && tt.wantFallback && resp.ID != "fallback" {
				t.Errorf("resp.ID = %q, want %q", resp.ID, "fallback")
			}
		})
	}
}

func TestFallbackProviderChatCompletionStream(t *testing.T) {
	primary := &recordingProvider{err: &UpstreamError{StatusCode: 503}}
	fallback := &recordingProvider{}
	p := FallbackProvider{Primary: primary, Fallback: fallback, FallbackModel: "qwen"}

	stream, err := p.ChatCompletionStream(context.Background(), ChatCompletionRequest{Model: "gpt-4o"})

	if err != nil || stream == nil {
		t.Fatalf("stream, err = %v, %v, want a stream from the fallback", stream, err)
	}
	if fallback.gotModel != "qwen" {
		t.Errorf("fallback model = %q, want %q", fallback.gotModel, "qwen")
	}
}
