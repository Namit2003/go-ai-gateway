# Roadmap

## Phase 1 — Go Gateway Fundamentals

- [x] HTTP server
- [x] configuration
- [x] OpenAI-compatible request structs
- [x] provider interface
- [x] OpenAI provider
- [x] basic error handling
- [x] unit tests

## Phase 2 — Streaming

- [x] SSE streaming
- [x] client cancellation
- [ ] provider streaming abstraction
- [ ] disconnect handling
- [ ] graceful shutdown

## Phase 3 — Reliability

- [ ] timeouts
- [ ] retry policy
- [ ] exponential backoff
- [ ] circuit breaker
- [ ] fallback provider

## Phase 4 — Traffic Control

- [ ] rate limiter
- [ ] concurrency limiter
- [ ] quotas

## Phase 5 — Routing

- [ ] configurable routing rules
- [ ] latency-aware routing
- [ ] cost-aware routing



## Phase 6 — Observability

- [ ] OpenTelemetry
- [ ] Prometheus
- [ ] structured logging



## Phase 7 — Performance

- [ ] load generator
- [ ] benchmarks
- [ ] profiling
- [ ] failure injection