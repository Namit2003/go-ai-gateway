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
		w.Write([]byte(`{"id":"c1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hello","reasoning_content":"think"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
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
	if resp.ID != "c1" || resp.Choices[0].FinishReason != "stop" || resp.Choices[0].Message.ReasoningContent != "think" {
		t.Errorf("resp = %+v, want id, finish_reason and reasoning_content to be kept", resp)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 3 {
		t.Errorf("usage = %+v, want total_tokens 3", resp.Usage)
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
		w.Write([]byte("data: {}\n\n"))
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

	for err == nil {
		_, err = stream.Recv()
	}
	if err == io.EOF {
		t.Error("stream ended normally, want an error after cancel")
	}

	select {
	case <-upstreamDone:
	case <-time.After(time.Second):
		t.Error("upstream request was not cancelled")
	}
}

func TestOpenAIChatCompletionStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: {\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"think\"}}]}\n\n"))
		w.Write([]byte("data: {\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		w.Write([]byte("data: {\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		w.Write([]byte("data: {\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	o := OpenAI{BaseURL: upstream.URL}

	stream, err := o.ChatCompletionStream(context.Background(), ChatCompletionRequest{Model: "m", Stream: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stream.Close()

	var got, reasoning string
	var finishReason *string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got += chunk.Choices[0].Delta.Content
		reasoning += chunk.Choices[0].Delta.ReasoningContent
		if chunk.Choices[0].FinishReason != nil {
			finishReason = chunk.Choices[0].FinishReason
		}
	}
	if got != "Hello" {
		t.Errorf("content = %q, want %q", got, "Hello")
	}
	if reasoning != "think" {
		t.Errorf("reasoning = %q, want %q", reasoning, "think")
	}
	if finishReason == nil || *finishReason != "stop" {
		t.Errorf("finish_reason = %v, want %q", finishReason, "stop")
	}
}
