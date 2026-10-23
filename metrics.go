package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_requests_total",
		Help: "Chat completion requests by provider, upstream model and HTTP status.",
	}, []string{"provider", "model", "status"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_request_duration_seconds",
		Help:    "Chat completion request duration by provider.",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider"})
)
