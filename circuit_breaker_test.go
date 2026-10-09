package main

import (
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	b := NewCircuitBreaker(2, time.Hour)

	b.Record(true)
	if !b.Allow() {
		t.Fatal("Allow() = false after 1 failure, want true")
	}
	b.Record(true)
	if b.Allow() {
		t.Error("Allow() = true after 2 failures, want false")
	}
}

func TestCircuitBreakerSuccessResetsFailures(t *testing.T) {
	b := NewCircuitBreaker(2, time.Hour)

	b.Record(true)
	b.Record(false)
	b.Record(true)

	if !b.Allow() {
		t.Error("Allow() = false, want true: the success should have reset the count")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	b := NewCircuitBreaker(1, 50*time.Millisecond)
	b.Record(true)
	time.Sleep(60 * time.Millisecond)

	if !b.Allow() {
		t.Fatal("Allow() = false after cooldown, want true for the trial request")
	}
	if b.Allow() {
		t.Error("second Allow() = true while the trial is in flight, want false")
	}

	b.Record(true)
	if b.Allow() {
		t.Error("Allow() = true after a failed trial, want false")
	}

	time.Sleep(60 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("Allow() = false after second cooldown, want true")
	}
	b.Record(false)
	if !b.Allow() {
		t.Error("Allow() = false after a successful trial, want true")
	}
}
