package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type requestInfoKey struct{}

type requestInfo struct {
	Client        string
	Model         string
	UpstreamModel string
	Provider      string
}

func requestInfoFrom(ctx context.Context) *requestInfo {
	if info, ok := ctx.Value(requestInfoKey{}).(*requestInfo); ok {
		return info
	}
	return &requestInfo{}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	s.ResponseWriter.(http.Flusher).Flush()
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		info := &requestInfo{}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), requestInfoKey{}, info)))
		elapsed := time.Since(start)
		requestsTotal.WithLabelValues(info.Provider, info.UpstreamModel, strconv.Itoa(rec.status)).Inc()
		requestDuration.WithLabelValues(info.Provider).Observe(elapsed.Seconds())
		slog.Info("request",
			"client", info.Client,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", elapsed.Milliseconds(),
			"model", info.Model,
			"upstream_model", info.UpstreamModel,
			"provider", info.Provider,
		)
	})
}
