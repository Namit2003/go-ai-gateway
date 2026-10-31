package main

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	exploreRate    = 0.05
	failurePenalty = 10 * time.Second
)

type LatencyProvider struct {
	names     []string
	providers []Provider
	explore   float64

	mu  sync.Mutex
	avg []time.Duration
}

func NewLatencyProvider(names []string, providers []Provider) *LatencyProvider {
	return &LatencyProvider{
		names:     names,
		providers: providers,
		explore:   exploreRate,
		avg:       make([]time.Duration, len(providers)),
	}
}

func (l *LatencyProvider) pick() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	best := 0
	for i, a := range l.avg {
		if a == 0 {
			return i
		}
		if a < l.avg[best] {
			best = i
		}
	}
	if rand.Float64() < l.explore {
		return rand.IntN(len(l.avg))
	}
	return best
}

func (l *LatencyProvider) observe(i int, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	d = max(d, 1)
	if l.avg[i] == 0 {
		l.avg[i] = d
		return
	}
	l.avg[i] = (4*l.avg[i] + d) / 5
}

func (l *LatencyProvider) record(ctx context.Context, i int, start time.Time, err error) {
	switch {
	case err == nil:
		l.observe(i, time.Since(start))
	case upstreamFailed(ctx, err):
		l.observe(i, failurePenalty)
	}
}

func (l *LatencyProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	i := l.pick()
	requestInfoFrom(ctx).Provider = l.names[i]
	start := time.Now()
	resp, err := l.providers[i].ChatCompletion(ctx, req)
	l.record(ctx, i, start, err)
	return resp, err
}

func (l *LatencyProvider) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
	i := l.pick()
	requestInfoFrom(ctx).Provider = l.names[i]
	start := time.Now()
	stream, err := l.providers[i].ChatCompletionStream(ctx, req)
	l.record(ctx, i, start, err)
	return stream, err
}
