package main

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker open")

type CircuitBreaker struct {
	mu            sync.Mutex
	threshold     int
	cooldown      time.Duration
	failures      int
	openedAt      time.Time
	trialInFlight bool
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{threshold: threshold, cooldown: cooldown}
}

func (b *CircuitBreaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.failures < b.threshold {
		return true
	}
	if time.Since(b.openedAt) < b.cooldown || b.trialInFlight {
		return false
	}
	b.trialInFlight = true
	return true
}

func (b *CircuitBreaker) Record(failed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.trialInFlight = false
	if !failed {
		b.failures = 0
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.openedAt = time.Now()
	}
}
