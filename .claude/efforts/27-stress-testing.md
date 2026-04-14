# Stress Testing Framework

**Status:** Planning
**Started:** 2026-04-13

---

## Goal

Build a stress testing framework to verify OpenDecree's behavior under load. Systematically catch resource exhaustion bugs (like unbounded caches, #107) before they reach production.

## Areas to Cover

### 1. Cache Pressure
- **Many tenants** — simulate hundreds of tenants creating schemas and configs to stress MemoryCache and ValidatorCache.
- **Many fields per schema** — schemas with large field counts to test per-schema memory footprint.
- **Rapid config changes** — high-frequency SetConfig calls to stress cache invalidation and Redis pub/sub propagation.
- **Cache eviction behavior** — verify LRU/max-size limits actually evict entries under pressure.

### 2. Connection Pool Exhaustion
- **PostgreSQL** — saturate the PG connection pool with concurrent queries. Verify graceful degradation (errors, not hangs).
- **Redis** — saturate Redis connections with concurrent cache reads, pub/sub subscriptions, and lock operations.
- **Connection leak detection** — run sustained load and verify pool sizes remain stable over time.

### 3. Large Payload Handling
- **Big schemas** — schemas with hundreds of fields, deeply nested groups, complex constraints.
- **Large JSON values** — config fields with large JSON blobs (KB to MB range).
- **Bulk operations** — large ListConfigs / ListSchemas responses, large import payloads.
- **Validation cost** — complex JSON Schema constraints on large values.

### 4. Concurrent Tenant Operations
- **Parallel reads/writes** — multiple tenants performing reads and writes simultaneously.
- **Lock contention** — concurrent SetConfig calls for the same tenant/schema to stress distributed locking.
- **Cross-tenant isolation under load** — verify tenant data never leaks even under concurrent pressure.

### 5. Memory Growth Profiling
- **pprof integration** — expose pprof endpoints in test harness for heap, goroutine, and alloc profiling.
- **Baseline measurement** — capture memory baseline, run sustained load, verify growth is bounded.
- **Goroutine leak detection** — verify goroutine count returns to baseline after load completes.

### 6. Pagination Under Load
- **Many tenants** — ListTenants pagination with hundreds of tenants.
- **Many schemas per tenant** — ListSchemas pagination with large schema counts.
- **Concurrent pagination** — multiple clients paginating simultaneously.

## Approach

### Phase 1: Framework + Cache Pressure Tests
- [ ] Create `stress/` package with test harness (setup/teardown, helpers, metrics collection)
- [ ] Use Go's `testing.B` benchmarks as the execution model — no external tools (vanilla principle)
- [ ] Implement cache pressure tests: many tenants, many fields, rapid config changes
- [ ] Verify cache eviction and memory bounds
- [ ] Add Makefile target: `make stress`

### Phase 2: Connection Pool + Payload Limits
- [ ] Add connection pool exhaustion tests (PG and Redis)
- [ ] Add large payload tests (big schemas, large JSON values, bulk operations)
- [ ] Add connection leak detection (sustained load, pool size monitoring)
- [ ] Add validation cost benchmarks for complex constraints

### Phase 3: Memory Profiling + Growth Tracking
- [ ] Integrate pprof into test harness for automated heap snapshots
- [ ] Add goroutine leak detection (before/after goroutine count comparison)
- [ ] Add memory growth benchmarks with bounded-growth assertions
- [ ] Add concurrent tenant operation tests with isolation verification
- [ ] Document profiling workflow and thresholds

## Design Principles

- **Go-native** — `testing.B` benchmarks + custom harness. No external load testing tools (k6, vegeta, etc.).
- **Deterministic** — tests use fixed seed data and controlled concurrency for reproducible results.
- **CI-friendly** — stress tests run in Docker Compose like e2e tests. Short mode for CI, full mode for manual runs.
- **Actionable** — tests assert specific bounds (max memory, max latency, no leaks), not just report numbers.

## Related

- #107 — Unbounded caches (discovered this need)
- Effort 25 — Security review (overlaps on input validation and resource exhaustion)
