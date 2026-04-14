# E2E Benchmarks

**Status:** Complete
**Parent:** 10-benchmarks

---

## Goal

System-level performance benchmarks against the real service stack (PG + Redis + service). Measure gRPC latency, throughput, and identify bottlenecks under load.

## What to Benchmark

### Latency (p50, p95, p99)
- `GetField` — single field read (cache hit vs miss)
- `GetConfig` — full config read
- `SetField` — single field write (includes audit + cache invalidation)
- `SetFields` — batch write (5, 10, 50 fields)
- `GetForUpdate` + `SetField` — CAS round-trip

### Throughput (requests/second)
- Read-heavy workload (90% reads, 10% writes)
- Write-heavy workload (50/50)
- Pure read workload (100% reads, cache warm)

### Specific scenarios
- Cold cache vs warm cache read comparison
- Subscription event propagation latency (write → subscriber receives change)
- Config import time vs field count (10, 50, 100, 500 fields)
- Concurrent readers during writes (read consistency under contention)
- Validator overhead: constrained writes vs unconstrained

## Approach Options

### Option A: Go test benchmarks against live service

Standard `testing.B` benchmarks in the `e2e/` module, run against docker-compose stack.

```go
func BenchmarkGetField(b *testing.B) {
    conn := dialBench(b)
    cfg := configclient.New(pb.NewConfigServiceClient(conn))
    // setup: create schema, tenant, set a value
    b.ResetTimer()
    for b.Loop() {
        cfg.Get(context.Background(), tenantID, "field")
    }
}
```

**Pros:** Native Go, same tooling, `benchstat` compatible, easy to add.
**Cons:** Single-client perspective, no concurrent load testing built-in.

### Option B: ghz (gRPC benchmarking tool)

[ghz](https://ghz.sh/) — dedicated gRPC load testing tool. Configurable concurrency, duration, rate limiting.

```bash
ghz --insecure --proto proto/centralconfig/v1/config_service.proto \
    --call centralconfig.v1.ConfigService/GetField \
    --data '{"tenant_id":"...","field_path":"..."}' \
    --concurrency 50 --duration 30s \
    localhost:9090
```

**Pros:** Built for load testing, nice reports (latency histogram, percentiles), configurable concurrency.
**Cons:** External tool, proto file needed, harder to script complex scenarios (multi-step flows).

### Option C: k6 with gRPC plugin

[k6](https://k6.io/) — general load testing tool with gRPC support.

```javascript
import grpc from 'k6/net/grpc';
const client = new grpc.Client();
client.load(['proto'], 'config_service.proto');

export default function () {
    client.connect('localhost:9090', { plaintext: true });
    client.invoke('centralconfig.v1.ConfigService/GetField', { ... });
}
```

**Pros:** Complex scenarios (ramp-up, stages, thresholds), cloud integration, HTML reports.
**Cons:** JavaScript, not Go. Heavier dependency. Overkill for initial benchmarks.

### Option D: Go benchmarks with parallel sub-benchmarks

Extended version of Option A with `b.RunParallel` for concurrent load:

```go
func BenchmarkGetField_Parallel(b *testing.B) {
    // setup...
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            cfg.Get(ctx, tenantID, "field")
        }
    })
}
```

**Pros:** Native Go, tests concurrency, GOMAXPROCS-aware, benchstat compatible.
**Cons:** Still single-machine load generation.

## Recommendation

**Start with Option A + D** (Go benchmarks, sequential + parallel). Reasons:
- Same tooling and language as everything else (vanilla principle)
- `benchstat` for regression detection
- `b.RunParallel` gives us real concurrency numbers
- No external tools needed
- Easy to run: `make bench-e2e`

**Add ghz later** if we need higher-concurrency load profiles or nicer reports for stakeholders.

## Infrastructure

Benchmarks run against the same docker-compose stack as e2e tests. Dedicated `make bench-e2e` target:

```makefile
bench-e2e:
    docker compose up -d --wait service
    cd e2e && go test -tags=e2e -bench=. -benchmem -count=3 -timeout=300s ./...
    docker compose down -v
```

## Implementation Plan

- [x] Benchmark helpers (benchEnv setup/teardown, dialBench)
- [x] Read latency benchmarks (GetField, GetAll)
- [x] Write latency benchmarks (SetField, SetInt)
- [x] Parallel throughput benchmarks (GetField, GetAll, SetField)
- [x] Mixed workload (90% read / 10% write)
- [x] Import benchmark (10 and 50 fields)
- [x] CAS round-trip (GetForUpdate + Set)
- [x] Snapshot read benchmark
- [x] `make bench` and `make bench-e2e` targets in Makefile
- Note: Subscription propagation benchmark deferred (requires async stream setup in bench context)
