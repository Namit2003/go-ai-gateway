package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
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
			for {
				chunk, err := stream.Recv()
				if err == io.EOF {
					fmt.Fprint(w, "data: [DONE]\n\n")
					flusher.Flush()
					return
				}
				if err != nil {
					if r.Context().Err() != nil {
						log.Printf("client disconnected")
					} else {
						log.Printf("stream failed: %v", err)
					}
					return
				}
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
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
	if errors.Is(err, ErrCircuitOpen) {
		writeError(w, http.StatusServiceUnavailable, "upstream_error", "upstream temporarily unavailable")
		return
	}
	if errors.Is(err, ErrNoRoute) {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "no route matches this request")
		return
	}
	var upErr *UpstreamError
	if errors.As(err, &upErr) && upErr.StatusCode == http.StatusTooManyRequests {
		writeError(w, http.StatusTooManyRequests, "rate_limit_error", "upstream rate limit exceeded")
		return
	}
	writeError(w, http.StatusBadGateway, "upstream_error", "upstream request failed")
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	cfg := LoadConfig()
	if len(os.Args) == 3 && os.Args[1] == "newkey" {
		key, err := newKey(cfg.APIKeysFile, os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("key for %s (shown only once): %s\n", os.Args[2], key)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	maxAttempts = cfg.MaxAttempts
	upstreamClient = newUpstreamClient(cfg.HeaderTimeout)
	var provider Provider = OpenAI{APIKey: cfg.OpenAIKey, BaseURL: cfg.OpenAIBaseURL, Breaker: NewCircuitBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown)}
	router, err := LoadRouter(cfg.RoutesFile, provider, cfg)
	if err != nil {
		log.Fatal(err)
	}
	provider = router
	if cfg.FallbackBaseURL != "" {
		provider = FallbackProvider{
			Primary:       provider,
			Fallback:      OpenAI{APIKey: cfg.FallbackAPIKey, BaseURL: cfg.FallbackBaseURL, Breaker: NewCircuitBreaker(cfg.BreakerThreshold, cfg.BreakerCooldown)},
			FallbackModel: cfg.FallbackModel,
		}
	}

	keys, err := LoadAPIKeys(cfg.APIKeysFile)
	if err != nil {
		log.Fatal(err)
	}
	if len(keys) == 0 {
		log.Printf("warning: no API keys in %s, all requests will be rejected (create one with: make newkey NAME=<name>)", cfg.APIKeysFile)
	}
	limiter := NewClientRateLimiter(ctx, rate.Limit(cfg.RateLimit), cfg.RateBurst)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.Handle("GET /metrics", promhttp.Handler())

	mux.Handle("POST /v1/chat/completions", logRequests(requireAPIKey(keys, rateLimit(limiter, limitConcurrency(cfg.MaxConcurrent, quota(NewQuotaTracker(cfg.DailyQuota), chatCompletionsHandler(provider)))))))

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown failed: %v", err)
	}
}
