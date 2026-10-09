package main

import "net/http"

func limitConcurrency(max int, next http.Handler) http.Handler {
	sem := make(chan struct{}, max)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		default:
			writeError(w, http.StatusTooManyRequests, "rate_limit_error", "too many concurrent requests")
		}
	})
}
