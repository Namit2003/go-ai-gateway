package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
)

func chatCompletionsHandler(provider Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON")
			return
		}
		if req.Model == "" {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "model is required")
			return
		}
		if len(req.Messages) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "messages is required")
			return
		}
		if req.Stream {
			stream, err := provider.ChatCompletionStream(r.Context(), req)
			if err != nil {
				writeProviderError(w, err)
				return
			}
			defer stream.Close()

			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				fmt.Fprintln(w, scanner.Text())
				flusher.Flush()
			}
			return
		}
		resp, err := provider.ChatCompletion(r.Context(), req)
		if err != nil {
			writeProviderError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func writeProviderError(w http.ResponseWriter, err error) {
	log.Printf("chat completion failed: %v", err)
	var upErr *UpstreamError
	if errors.As(err, &upErr) && upErr.StatusCode == http.StatusTooManyRequests {
		writeError(w, http.StatusTooManyRequests, "rate_limit_error", "upstream rate limit exceeded")
		return
	}
	writeError(w, http.StatusBadGateway, "upstream_error", "upstream request failed")
}

func main() {
	cfg := LoadConfig()
	var provider Provider = OpenAI{APIKey: cfg.OpenAIKey, BaseURL: cfg.OpenAIBaseURL}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /v1/chat/completions", chatCompletionsHandler(provider))

	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}
