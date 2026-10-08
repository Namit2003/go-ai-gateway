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

var upstreamClient = func() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 60 * time.Second
	return &http.Client{Transport: t}
}()

type OpenAI struct {
	APIKey  string
	BaseURL string
}

func (o OpenAI) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	var out ChatCompletionResponse

	body, err := json.Marshal(req)
	if err != nil {
		return out, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := upstreamClient.Do(httpReq)
	if err != nil {
		return out, fmt.Errorf("calling openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return out, &UpstreamError{StatusCode: resp.StatusCode}
	}

	err = json.NewDecoder(resp.Body).Decode(&out)
	return out, err
}

func (o OpenAI) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := upstreamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling openai: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &UpstreamError{StatusCode: resp.StatusCode}
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
