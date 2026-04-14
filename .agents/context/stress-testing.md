# Stress Testing — Design Context

## Areas to Cover

1. **Cache pressure** — many tenants, many fields, rapid config changes, eviction behavior
2. **Connection pool exhaustion** — PG and Redis saturation, leak detection
3. **Large payloads** — big schemas, large JSON values, bulk operations, validation cost
4. **Concurrent operations** — parallel reads/writes, lock contention, cross-tenant isolation under load
5. **Memory profiling** — pprof integration, baseline measurement, goroutine leak detection
6. **Pagination under load** — many tenants/schemas, concurrent pagination

## Design Principles

- **Go-native** — `testing.B` benchmarks + custom harness. No external tools (k6, vegeta).
- **Deterministic** — fixed seed data, controlled concurrency, reproducible results.
- **CI-friendly** — Docker Compose like e2e tests. Short mode for CI, full mode for manual.
- **Actionable** — assert specific bounds (max memory, max latency, no leaks), not just report.
