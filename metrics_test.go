package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMetrics(t *testing.T) {
	captureLogs(t)
	router := &Router{Default: &recordingProvider{}, routes: []route{
		{Rule{Model: "auto", Provider: "metrics-test", UseModel: "metrics-model"}, &recordingProvider{}},
	}}
	h := logRequests(chatCompletionsHandler(router))
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"auto","messages":[{"role":"user","content":"hi"}]}`))
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)

	for _, want := range []string{
		`gateway_requests_total{model="metrics-model",provider="metrics-test",status="200"} 2`,
		`gateway_request_duration_seconds_count{provider="metrics-test"} 2`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("/metrics is missing %s", want)
		}
	}
}
