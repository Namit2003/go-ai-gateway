package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIChatCompletionSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/chat/completions")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer secret")
		}
		var got ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		if got.Model != "m" {
			t.Errorf("model = %q, want %q", got.Model, "m")
		}
		w.Write([]byte(`{"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hello"}}]}`))
	}))
	defer upstream.Close()

	o := OpenAI{APIKey: "secret", BaseURL: upstream.URL}
	req := ChatCompletionRequest{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}

	resp, err := o.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "hello" {
		t.Errorf("resp = %+v, want one choice with content %q", resp, "hello")
	}
}

func TestOpenAIChatCompletionUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	o := OpenAI{APIKey: "bad", BaseURL: upstream.URL}

	_, err := o.ChatCompletion(context.Background(), ChatCompletionRequest{Model: "m"})

	var upErr *UpstreamError
	if !errors.As(err, &upErr) {
		t.Fatalf("err = %v, want an *UpstreamError", err)
	}
	if upErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", upErr.StatusCode, http.StatusUnauthorized)
	}
}

func TestOpenAIChatCompletionStreamCancel(t *testing.T) {
	upstreamDone := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: hi\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(upstreamDone)
	}))
	defer upstream.Close()

	ctx, cancel := context.WithCancel(context.Background())
	o := OpenAI{BaseURL: upstream.URL}

	stream, err := o.ChatCompletionStream(ctx, ChatCompletionRequest{Model: "m", Stream: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stream.Close()

	cancel()

	if _, err := io.ReadAll(stream); err == nil {
		t.Error("reading the stream after cancel returned no error")
	}

	select {
	case <-upstreamDone:
	case <-time.After(time.Second):
		t.Error("upstream request was not cancelled")
	}
}
