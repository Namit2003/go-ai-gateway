package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func post(t *testing.T, h http.Handler, body string) (*http.Response, error) {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return http.Post(srv.URL, "application/json", strings.NewReader(body))
}

func TestHandlerHealthy(t *testing.T) {
	resp, err := post(t, newHandler(0, 0, 0), `{"model":"m"}`)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("resp = %v, err = %v, want 200", resp, err)
	}
}

func TestHandlerInjectsErrors(t *testing.T) {
	resp, err := post(t, newHandler(1, 0, 0), `{"model":"m"}`)
	if err != nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("resp = %v, err = %v, want 503", resp, err)
	}
}

func TestHandlerDropsConnections(t *testing.T) {
	if resp, err := post(t, newHandler(0, 1, 0), `{"model":"m"}`); err == nil {
		t.Errorf("got status %d, want a connection error", resp.StatusCode)
	}
}

func TestHandlerDelays(t *testing.T) {
	start := time.Now()
	post(t, newHandler(0, 0, 100*time.Millisecond), `{"model":"m"}`)
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("responded after %v, want at least 100ms", elapsed)
	}
}
