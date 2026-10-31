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
- [x] provider streaming abstraction
- [x] disconnect handling
- [x] graceful shutdown

## Phase 3 — Reliability

- [x] timeouts
- [x] retry policy
- [x] exponential backoff
- [x] circuit breaker
- [x] fallback provider

## Phase 4 — Traffic Control

- [x] rate limiter
- [x] concurrency limiter
- [x] quotas

## Phase 5 — Routing

- [x] configurable routing rules
- [x] latency-aware routing
- [ ] cost-aware routing



## Phase 6 — Observability

- [x] Prometheus
- [x] structured logging



## Phase 7 — Performance

- [x] load generator
- [x] benchmarks
- [x] profiling
- [x] failure injection

## Phase 8 — Later

- [ ] native Anthropic provider (/v1/messages)
- [x] token-based quota
- [ ] OpenTelemetry
