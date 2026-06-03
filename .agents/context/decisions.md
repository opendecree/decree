# Architecture Decisions

---

## Functional-options boilerplate (issue #298)

**Date:** 2026-06-03

**Question:** Should we introduce a generic `internal/optutil` helper to deduplicate `WithLogger` and `Option` definitions across 6+ packages?

**Survey:**

All 6 `WithLogger` functions follow an identical 3-line structure but differ only in their return type and inner `opts` struct name:

| Package | Return type | Inner struct |
|---------|-------------|--------------|
| `internal/auth/jwt.go` | `InterceptorOption` | `*interceptorOptions` |
| `internal/audit/recorder.go` | `Option` | `*recorderOptions` |
| `internal/pubsub/memory.go` | `MemoryOption` | `*MemoryPubSub` |
| `internal/schema/service.go` | `Option` | `*serviceOptions` |
| `internal/server/options.go` | `Option` | `*options` |
| `internal/config/service.go` | `Option` | `*serviceOptions` |

Each `WithLogger` is exactly:
```go
func WithLogger(l *slog.Logger) <PackageOption> {
    return func(o *<pkgOpts>) { o.logger = l }
}
```

The proposed generic helper (`internal/optutil`) would require each package's `opts` struct to implement a `setLogger(*slog.Logger)` method, replacing the 3-line `WithLogger` with a 2-line method. Net line change: zero reduction, with added coupling via a new shared import.

Additional option types surveyed (no `WithLogger`):
- `internal/validation/limits.go` — `Option` (4 distinct option constructors: `WithLimits`, `WithNullable`, `WithSensitive`, `WithConstraints`)
- `internal/ratelimit/interceptor.go` — `Option`
- `internal/storage/postgres.go` — `Option`
- `sdk/adminclient/options.go` — `Option`

These packages do not share any option constructors with each other, confirming the duplication is shallow (logger only) rather than pervasive.

**Decision:** Accept the duplication.

**Rationale:**
- In Go, the `Option` function type (`func(*opts)`) must be per-package for type safety — callers cannot mix options from different packages, and the compiler enforces this naturally via distinct named types.
- A generic `WithLoggerFor[T interface{ setLogger(*slog.Logger) }]` helper would trade each 3-line `WithLogger` for a 2-line `setLogger` method — no net reduction in lines, and it adds coupling via an `internal/optutil` import across 6 packages.
- Code generation (e.g., go generate templates) would eliminate the repetition but adds a mandatory build step; rejected per Vanilla Principle (only standard/widely-adopted tools).
- The 6 `WithLogger` definitions are each 3 lines, trivially readable, and never drift independently — the logger field and its assignment are the same pattern across all packages.
- The per-package `Option` types differ materially beyond `WithLogger` (each package has its own set of option constructors); a shared base would not unify those.
- Closed without code changes.
