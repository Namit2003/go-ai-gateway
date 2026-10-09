package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLimitConcurrency(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	h := limitConcurrency(2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
	}))

	done := make(chan struct{})
	for range 2 {
		go func() {
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
			done <- struct{}{}
		}()
		<-started
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request over the limit: status = %d, want 429", rec.Code)
	}

	close(release)
	<-done
	<-done

	go func() { <-started }()
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("after slots were freed: status = %d, want 200", rec.Code)
	}
}
