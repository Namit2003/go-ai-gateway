package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestLatency(a, b Provider) *LatencyProvider {
	l := NewLatencyProvider([]string{"a", "b"}, []Provider{a, b})
	l.explore = 0
	return l
}

func TestLatencyProviderTriesEachBeforePickingFastest(t *testing.T) {
	a, b := &recordingProvider{}, &recordingProvider{}
	l := newTestLatency(a, b)
	ctx := context.Background()

	l.ChatCompletion(ctx, ChatCompletionRequest{Model: "m"})
	l.ChatCompletion(ctx, ChatCompletionRequest{Model: "m"})
	if !a.called || !b.called {
		t.Errorf("a.called = %v, b.called = %v, want both tried once", a.called, b.called)
	}
}

func TestLatencyProviderPicksFastest(t *testing.T) {
	l := newTestLatency(&recordingProvider{}, &recordingProvider{})
	l.avg = []time.Duration{300 * time.Millisecond, 100 * time.Millisecond}

	if got := l.pick(); got != 1 {
		t.Errorf("pick = %d, want 1 (the faster provider)", got)
	}
}

func TestLatencyProviderFailureCountsAsSlow(t *testing.T) {
	a := &recordingProvider{err: errors.New("boom")}
	l := newTestLatency(a, &recordingProvider{})
	l.avg = []time.Duration{time.Millisecond, 50 * time.Millisecond}

	for range 3 {
		l.ChatCompletion(context.Background(), ChatCompletionRequest{Model: "m"})
	}
	if got := l.pick(); got != 1 {
		t.Errorf("pick = %d, want 1 after the fast provider kept failing", got)
	}
}

func TestLatencyProviderClientCancelIsNotRecorded(t *testing.T) {
	a := &recordingProvider{err: context.Canceled}
	l := newTestLatency(a, &recordingProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	l.ChatCompletion(ctx, ChatCompletionRequest{Model: "m"})
	if l.avg[0] != 0 {
		t.Errorf("avg = %v, want no sample for a cancelled request", l.avg[0])
	}
}

func TestLatencyProviderObserveAverages(t *testing.T) {
	l := newTestLatency(&recordingProvider{}, &recordingProvider{})
	l.observe(0, 100*time.Millisecond)
	l.observe(0, 200*time.Millisecond)

	if want := 120 * time.Millisecond; l.avg[0] != want {
		t.Errorf("avg = %v, want %v", l.avg[0], want)
	}
}

func TestLatencyProviderExploresSlowerProvider(t *testing.T) {
	l := newTestLatency(&recordingProvider{}, &recordingProvider{})
	l.explore = 1
	l.avg = []time.Duration{time.Millisecond, time.Second}

	for range 200 {
		if l.pick() == 1 {
			return
		}
	}
	t.Error("the slower provider was never picked with explore = 1")
}

func TestLatencyProviderSetsRequestInfoProvider(t *testing.T) {
	l := newTestLatency(&recordingProvider{}, &recordingProvider{})
	info := &requestInfo{}
	ctx := context.WithValue(context.Background(), requestInfoKey{}, info)

	l.ChatCompletion(ctx, ChatCompletionRequest{Model: "m"})
	if info.Provider != "a" {
		t.Errorf("Provider = %q, want a", info.Provider)
	}
}
