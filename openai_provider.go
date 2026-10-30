package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const retryBaseDelay = 100 * time.Millisecond

var maxAttempts = 3

var upstreamClient = newUpstreamClient(60 * time.Second)

func newUpstreamClient(headerTimeout time.Duration) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = headerTimeout
	t.MaxIdleConnsPerHost = 100
	return &http.Client{Transport: t}
}

type OpenAI struct {
	APIKey  string
	BaseURL string
	Breaker *CircuitBreaker
}

func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func (o OpenAI) post(ctx context.Context, body []byte) (*http.Response, error) {
	if o.Breaker == nil {
		return o.postWithRetries(ctx, body)
	}
	if !o.Breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	resp, err := o.postWithRetries(ctx, body)

	o.Breaker.Record(upstreamFailed(ctx, err))
	return resp, err
}

func (o OpenAI) postWithRetries(ctx context.Context, body []byte) (*http.Response, error) {
	var lastErr error
	for attempt := range maxAttempts {
		if attempt > 0 {
			select {
			case <-time.After(retryBaseDelay << (attempt - 1)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := upstreamClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("calling openai: %w", err)
			if ctx.Err() != nil {
				return nil, lastErr
			}
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		resp.Body.Close()
		lastErr = &UpstreamError{StatusCode: resp.StatusCode}
		if !retryableStatus(resp.StatusCode) {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

func (o OpenAI) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	var out ChatCompletionResponse

	body, err := json.Marshal(req)
	if err != nil {
		return out, err
	}

	resp, err := o.post(ctx, body)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&out)
	return out, err
}

func (o OpenAI) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := o.post(ctx, body)
	if err != nil {
		return nil, err
	}

	return &openAIStream{body: resp.Body, scanner: bufio.NewScanner(resp.Body)}, nil
}

type openAIStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
}

func (s *openAIStream) Recv() (ChatCompletionChunk, error) {
	var chunk ChatCompletionChunk
	for s.scanner.Scan() {
		data, ok := strings.CutPrefix(s.scanner.Text(), "data: ")
		if !ok {
			continue
		}
		if data == "[DONE]" {
			return chunk, io.EOF
		}
		err := json.Unmarshal([]byte(data), &chunk)
		return chunk, err
	}
	if err := s.scanner.Err(); err != nil {
		return chunk, err
	}
	return chunk, io.EOF
}

func (s *openAIStream) Close() error {
	return s.body.Close()
}
