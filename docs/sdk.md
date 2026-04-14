# SDKs

Three Go SDK packages, each an independent module. Full API reference is hosted on pkg.go.dev.

## configclient

Runtime config reads and writes for application code.

- **Install:** `go get github.com/opendecree/decree/sdk/configclient@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/configclient](https://pkg.go.dev/github.com/opendecree/decree/sdk/configclient)

Features: Get, GetAll, GetFields, Set, SetMany, typed getters/setters (GetInt, SetBool, etc.), Snapshot for pinned-version reads, GetForUpdate + Update for optimistic concurrency, null support.

## adminclient

Schema, tenant, audit, and config admin operations for tooling and CI/CD.

- **Install:** `go get github.com/opendecree/decree/sdk/adminclient@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/adminclient](https://pkg.go.dev/github.com/opendecree/decree/sdk/adminclient)

Features: Schema CRUD, publish, import/export (YAML), tenant CRUD, field locks, config versioning, rollback, audit log queries, usage stats.

## configwatcher

Live typed configuration values with automatic subscription and reconnect.

- **Install:** `go get github.com/opendecree/decree/sdk/configwatcher@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/configwatcher](https://pkg.go.dev/github.com/opendecree/decree/sdk/configwatcher)

Features: `Value[T]` generic type with `Get()`, `GetWithNull()`, `Changes()` channel. Typed accessors (String, Int, Float, Bool, Duration). Auto-reconnect with exponential backoff. Thread-safe.
