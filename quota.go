package main

import (
	"net/http"
	"sync"
	"time"
)

type quotaUsage struct {
	day   time.Time
	count int
}

type QuotaTracker struct {
	mu    sync.Mutex
	limit int
	now   func() time.Time
	usage map[string]*quotaUsage
}

func NewQuotaTracker(limit int) *QuotaTracker {
	return &QuotaTracker{limit: limit, now: time.Now, usage: make(map[string]*quotaUsage)}
}

func (q *QuotaTracker) Allow(name string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	day := q.now().UTC().Truncate(24 * time.Hour)
	u := q.usage[name]
	if u == nil || !u.day.Equal(day) {
		u = &quotaUsage{day: day}
		q.usage[name] = u
	}
	if u.count >= q.limit {
		return false
	}
	u.count++
	return true
}

func quota(q *QuotaTracker, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, _ := r.Context().Value(clientKey{}).(string)
		if !q.Allow(name) {
			writeError(w, http.StatusTooManyRequests, "insufficient_quota", "daily quota exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
