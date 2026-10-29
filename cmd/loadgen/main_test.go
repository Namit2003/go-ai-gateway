package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	var sorted []time.Duration
	for i := 1; i <= 100; i++ {
		sorted = append(sorted, time.Duration(i)*time.Millisecond)
	}
	tests := []struct {
		p    float64
		want time.Duration
	}{
		{0, 1 * time.Millisecond},
		{0.5, 50 * time.Millisecond},
		{1, 100 * time.Millisecond},
	}
	for _, tt := range tests {
		if got := percentile(sorted, tt.p); got != tt.want {
			t.Errorf("percentile(%v) = %v, want %v", tt.p, got, tt.want)
		}
	}
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("percentile of no data = %v, want 0", got)
	}
}

func TestRun(t *testing.T) {
	var served atomic.Int64
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		gotAuth.Store(r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	res := run(srv.URL, "sk-test", `{}`, 3, 100*time.Millisecond)

	if int64(len(res.latencies)) != served.Load() || served.Load() == 0 {
		t.Errorf("recorded %d latencies, server saw %d requests", len(res.latencies), served.Load())
	}
	if res.statuses[200] != len(res.latencies) {
		t.Errorf("statuses = %v, want all 200", res.statuses)
	}
	if gotAuth.Load() != "Bearer sk-test" {
		t.Errorf("Authorization = %v, want Bearer sk-test", gotAuth.Load())
	}
}

func TestRunCountsConnectionErrors(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()

	res := run(srv.URL, "k", `{}`, 1, 50*time.Millisecond)

	if res.statuses[0] == 0 || len(res.statuses) != 1 {
		t.Errorf("statuses = %v, want only status 0 (connection error)", res.statuses)
	}
}
