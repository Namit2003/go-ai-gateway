package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func TestLogRequests(t *testing.T) {
	buf := captureLogs(t)
	keys := map[string]string{hashKey("sk-good"): "alice"}
	router := &Router{Default: &recordingProvider{}, DefaultName: "default", routes: []route{
		{Rule{Model: "auto", Provider: "anthropic", UseModel: "claude-haiku"}, &recordingProvider{}},
	}}
	h := logRequests(requireAPIKey(keys, chatCompletionsHandler(router)))

	send := func(token, body string) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	send("sk-good", `{"model":"auto","messages":[{"role":"user","content":"hi"}]}`)
	send("sk-bad", `{}`)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d log lines, want 2: %s", len(lines), buf.String())
	}
	var ok, rejected map[string]any
	json.Unmarshal([]byte(lines[0]), &ok)
	json.Unmarshal([]byte(lines[1]), &rejected)

	want := map[string]any{
		"msg": "request", "client": "alice", "method": "POST", "path": "/v1/chat/completions",
		"status": float64(200), "model": "auto", "upstream_model": "claude-haiku", "provider": "anthropic",
	}
	for k, v := range want {
		if ok[k] != v {
			t.Errorf("%s = %v, want %v", k, ok[k], v)
		}
	}
	if rejected["status"] != float64(401) || rejected["client"] != "" || rejected["provider"] != "" {
		t.Errorf("rejected request log = %v, want status 401 and empty client and provider", rejected)
	}
}

func TestStatusRecorderFlush(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()
	h := logRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
	}))
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !rec.Flushed {
		t.Error("Flush did not reach the underlying writer")
	}
}

func TestLogRequestsFallback(t *testing.T) {
	buf := captureLogs(t)
	primary := &Router{Default: &recordingProvider{err: &UpstreamError{StatusCode: http.StatusServiceUnavailable}}, DefaultName: "default"}
	h := logRequests(chatCompletionsHandler(FallbackProvider{Primary: primary, Fallback: &recordingProvider{}, FallbackModel: "qwen"}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	h.ServeHTTP(httptest.NewRecorder(), req)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var line map[string]any
	json.Unmarshal([]byte(lines[len(lines)-1]), &line)
	if line["provider"] != "fallback" || line["upstream_model"] != "qwen" || line["model"] != "gpt-4o" {
		t.Errorf("log = %v, want provider fallback, upstream_model qwen, model gpt-4o", line)
	}
}
