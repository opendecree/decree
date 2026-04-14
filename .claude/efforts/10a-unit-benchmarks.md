# Unit Benchmarks

**Status:** Complete
**Parent:** 10-benchmarks

---

## Goal

Micro-benchmarks for hot-path functions using Go's `testing.B`. Run with `go test -bench=. -benchmem`. No external dependencies needed.

## What to Benchmark

### Conversion layer
- `typedValueToString` — per type (string, int, number, bool, time, duration)
- `stringToTypedValue` — per type
- `typedValueToDisplayString` — used in audit/events on every write

### Validation
- `FieldValidator.Validate` — per type, with and without constraints
- `NewFieldValidator` — construction cost (relevant for cache miss)
- JSON Schema validation — compile + validate (known to be expensive)
- Regex pattern matching

### Checksums
- `computeChecksum` — xxHash on value strings (stored in DB at write time)
- `typedValueChecksum` — checksum via TypedValue

### YAML
- `marshalSchemaYAML` / `unmarshalSchemaYAML` — small and large schemas
- `marshalConfigYAML` / `unmarshalConfigYAML` — small and large configs
- `configToYAML` / `yamlToConfigValues` — typed value conversion

### Cache
- `ValidatorCache.Get` — cache hit under contention (parallel benchmark)
- `ValidatorCache.Set` / `Invalidate` — write path

### Proto conversion
- `schemaToProto` / `fieldToProto` — DB → proto conversion
- `configVersionToProto`

## Approach

Standard Go benchmarks — `func BenchmarkXxx(b *testing.B)` in `_test.go` files alongside the code they benchmark.

```go
func BenchmarkTypedValueToString_Integer(b *testing.B) {
    tv := &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}
    for b.Loop() {
        typedValueToString(tv)
    }
}
```

### Options for organization

**Option A: Benchmarks in existing test files** — add `Benchmark*` functions to `convert_test.go`, `validator_test.go`, `yaml_test.go`. Pros: close to the code, easy to find. Cons: mixes unit tests and benchmarks.

**Option B: Separate benchmark files** — `convert_bench_test.go`, `validator_bench_test.go`. Pros: clean separation. Cons: more files.

**Option C: Dedicated benchmark package** — `internal/benchmark/`. Pros: single place. Cons: needs to import internal packages (possible via `_test` package trick but awkward).

### Recommendation: Option B

Separate `*_bench_test.go` files in each package. Clean, discoverable, standard Go practice.

## Running

```bash
# All benchmarks
go test ./internal/... -bench=. -benchmem -count=3

# Specific package
go test ./internal/config/... -bench=BenchmarkTypedValue -benchmem

# Compare before/after a change
go test ./internal/... -bench=. -benchmem -count=5 > old.txt
# ... make changes ...
go test ./internal/... -bench=. -benchmem -count=5 > new.txt
benchstat old.txt new.txt
```

## Implementation Plan

- [x] Conversion benchmarks — 8 benchmarks (`internal/config/convert_bench_test.go`)
- [x] Validation benchmarks — 9 benchmarks incl. cache parallel (`internal/validation/validator_bench_test.go`)
- [x] Config YAML benchmarks — 3 benchmarks (`internal/config/yaml_bench_test.go`)
- [x] Schema YAML benchmarks — 3 benchmarks (`internal/schema/yaml_bench_test.go`)
- [x] Cache benchmarks — included in validator_bench_test.go (Get_Hit, Get_Hit_Parallel)
- [x] `make bench` target in Makefile
