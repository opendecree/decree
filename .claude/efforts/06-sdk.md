# SDK

**Status:** Complete
**Started:** 2026-03-26

---

## Goal

Provide Go SDK packages for consuming the Central Config Service. Three layers, each a separate Go module, progressively higher-level:

1. **configclient** — Simple config read/write wrapper
2. **adminclient** — Schema/tenant/audit management wrapper
3. **configwatcher** — Live config with typed values, subscriptions, auto-reconnect

Each SDK is its own module under `sdk/` so consumers import only what they need. The watcher builds on configclient. All depend on the generated proto stubs in `api/`.

## Module Layout

```
go.work
├── api/                          # existing — generated proto stubs
├── sdk/
│   ├── configclient/             # sdk/configclient module
│   │   ├── go.mod
│   │   ├── client.go             # Client struct, constructor, connection mgmt
│   │   ├── options.go            # functional options (auth headers, retry, timeouts)
│   │   ├── read.go               # Get, GetAll, GetField, GetFields
│   │   ├── write.go              # Set, SetMany, Import, Export
│   │   └── client_test.go
│   ├── adminclient/              # sdk/adminclient module
│   │   ├── go.mod
│   │   ├── client.go             # Client struct, constructor
│   │   ├── schema.go             # Schema CRUD, publish, import/export
│   │   ├── tenant.go             # Tenant CRUD
│   │   ├── audit.go              # Audit queries
│   │   └── client_test.go
│   └── configwatcher/            # sdk/configwatcher module
│       ├── go.mod
│       ├── watcher.go            # Watcher struct, lifecycle
│       ├── value.go              # Value[T] typed accessor with changes channel
│       ├── types.go              # Type conversion (string ↔ native), null handling
│       ├── subscription.go       # gRPC stream management, reconnect, demux
│       └── watcher_test.go
```

## Sub-Efforts

### 06a — configclient
### 06b — adminclient
### 06c — configwatcher

See individual effort files for details.

## Shared Patterns

All SDKs share:
- **Connection management** — `grpc.ClientConn` passed in or created from target address
- **Auth headers** — `x-subject`, `x-role`, `x-tenant-id` injected via gRPC metadata (or Bearer token)
- **Context propagation** — all methods take `context.Context`
- **Error wrapping** — gRPC status errors unwrapped into meaningful Go errors

Shared code (if any) stays in a small internal package within sdk/, or is just duplicated if trivial.

## Dependency Direction

```
configwatcher → configclient → api (generated proto)
adminclient → api (generated proto)
```

The watcher uses configclient for initial config load and writes. It manages its own subscription stream directly via the generated proto client.

## Contrib Integrations

Third-party ecosystem integrations live under `contrib/`, each as its own Go module:

```
contrib/
├── viper/        # viper remote provider (future)
├── koanf/        # koanf provider (future)
└── ...
```

Dependency direction: `contrib/* → sdk/configclient or sdk/configwatcher → api`
