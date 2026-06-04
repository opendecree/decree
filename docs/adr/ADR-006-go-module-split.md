# ADR-006: Go Module Split (8 Modules)

**Date:** 2026-06-05
**Status:** Accepted
**Deciders:** OpenDecree maintainers

## Context

OpenDecree is a monorepo that contains both a server (with heavy infrastructure dependencies: gRPC server framework, PostgreSQL driver, Redis client, OTel SDK) and multiple client SDKs intended to be imported by end-user Go services.

If all code lived in a single Go module, SDK consumers would transitively pull in all server-side dependencies (database drivers, migration tools, server-only OTel exporters, etc.). This violates the principle of minimal consumer dependency footprint.

Additionally, different consumers have different Go version requirements:

- The server runs in a controlled build environment and can use the latest Go version
- CLI users installing via `go install` are likely on a recent but not necessarily bleeding-edge version
- SDK consumers may be on an older stable Go version (Go 1.22 was chosen as the lowest stable common ground)

A single module cannot simultaneously satisfy Go 1.22 compatibility for SDK consumers while using Go 1.25 language features in the server.

## Decision

The repository is split into 8 Go modules with a tiered Go version policy:

| Module | Path | Go version | Notes |
|--------|------|------------|-------|
| Server (root) | `.` | 1.25 | Full server binary, all infrastructure deps |
| API | `api/` | 1.24 | Generated proto stubs, consumed by transport + tools |
| CLI | `cmd/decree/` | 1.24 | User-facing CLI, matches transport floor |
| gRPC transport | `sdk/grpctransport/` | 1.24 | gRPC dial + interceptors for SDK clients |
| SDK tools | `sdk/tools/` | 1.24 | Shared SDK utilities, depends on api |
| Config client | `sdk/configclient/` | 1.22 | Lightweight SDK: read config values |
| Admin client | `sdk/adminclient/` | 1.22 | Lightweight SDK: manage schemas and tenants |
| Config watcher | `sdk/configwatcher/` | 1.22 | Lightweight SDK: stream config changes |

A `go.work` workspace file ties all modules together for local development and is **not committed to git** (gitignored), so consumers never see it.

During development, inter-module dependencies use `replace` directives in each module's `go.mod` to point at the local worktree. Published versions use normal module paths and semantic version tags.

## Consequences

**Positive:**

- SDK consumers (configclient, adminclient, configwatcher) import only lightweight modules with minimal transitive dependencies — no server-side infrastructure pulled in
- Each tier can declare its own minimum Go version, letting SDK consumers stay on Go 1.22 while the server uses Go 1.25 features
- Module boundaries make it structurally impossible for SDK code to accidentally import server internals

**Negative:**

- `replace` directives in `go.mod` files must be kept in sync during local development; a missing replace causes confusing "module not found" errors
- `go.work` is required for a working local development environment but is gitignored — new contributors must generate it (via `go work init && go work use ./...` or a `make` target)
- Releasing requires tagging 8 modules in the correct dependency order (leaves first)
- `go get` upgrades must be applied per module rather than globally
