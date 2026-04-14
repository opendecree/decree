# Contributing to Central Config Service

Thank you for your interest in contributing! This guide covers how to set up your development environment, build, test, and submit changes.

## Prerequisites

- **Go** (1.24+)
- **Docker** and **Docker Compose**
- **Make**

That's it. All other tools (buf, sqlc, goose, protoc plugins) run inside Docker — no local installation needed.

## Getting Started

```bash
# Clone the repository
git clone https://github.com/zeevdr/central-config-service.git
cd central-config-service

# Generate code from protobuf and SQL specs
make generate

# Run tests
make test

# Build the server and CLI
make build
```

## Development Cycle

The development workflow follows a specs-first approach:

```
modify specs → generate code → test → lint → build → deploy → e2e test
```

### Makefile Targets

| Target | Description | Runs in |
|--------|------------|---------|
| `make generate` | Generate Go code from protobuf (buf) and SQL (sqlc) | Docker |
| `make test` | Run unit tests | Local |
| `make lint` | Run golangci-lint, buf lint, buf breaking | Mixed |
| `make build` | Build the service binary to `bin/` | Local |
| `make image` | Build the Docker image | Docker |
| `make migrate` | Run database migrations | Docker |
| `make e2e` | Spin up full stack and run e2e tests | Docker |
| `make bench` | Run unit benchmarks | Local |
| `make bench-e2e` | Run e2e benchmarks against docker stack | Docker |
| `make clean` | Remove build artifacts and generated code | Local |
| `make all` | generate, lint, test, build | Mixed |

### Specs-First Workflow

1. **Protobuf** — edit `.proto` files under `proto/centralconfig/v1/`, then run `make generate`
2. **SQL queries** — edit `.sql` files under `db/queries/`, then run `make generate`
3. **Migrations** — edit `db/migrations/001_initial_schema.sql` (pre-production, single migration)

Generated code is **checked into git** — `go build` works immediately after cloning. Run `make generate` after modifying proto or SQL specs.

## Project Structure

The project uses a Go workspace (`go.work`) with multiple modules:

```
go.work                  # Go workspace definition
go.mod                   # Server module (cmd/server + internal/)

cmd/
├── server/              # Service entry point
└── ccs/                 # CLI tool (own module: cmd/ccs/go.mod)

proto/                   # Protobuf definitions (API source of truth)
api/                     # Generated proto Go code (own module: api/go.mod)
db/
├── queries/             # SQL queries (DB source of truth)
└── migrations/          # goose migrations

internal/
├── server/              # gRPC server setup, interceptors
├── schema/              # SchemaService implementation
├── config/              # ConfigService implementation
├── audit/               # AuditService implementation
├── auth/                # JWT + metadata auth
├── validation/          # Field constraint validation (factory + cache)
├── pubsub/              # Change propagation (Redis impl behind interface)
├── cache/               # Config cache (Redis impl behind interface)
├── storage/             # DB layer + sqlc generated code
└── telemetry/           # OpenTelemetry providers, metrics, slog handler

sdk/
├── configclient/        # Config read/write SDK (own module)
├── adminclient/         # Admin operations SDK (own module)
└── configwatcher/       # Live typed values SDK (own module)

e2e/                     # End-to-end tests (own module: e2e/go.mod)
build/                   # Dockerfiles (service + tools)
deploy/
├── helm/                # Helm chart
└── otel-collector.yaml  # OTel Collector config for local dev
contrib/                 # Third-party integrations (future: viper, koanf)
```

### Module Layout

| Module | Path | Purpose |
|--------|------|---------|
| Server | `.` | Service binary + internal packages |
| API | `api/` | Generated proto stubs (lightweight deps) |
| CLI | `cmd/ccs/` | CLI binary (depends on SDKs, not internals) |
| configclient | `sdk/configclient/` | Runtime config read/write SDK |
| adminclient | `sdk/adminclient/` | Admin management SDK |
| configwatcher | `sdk/configwatcher/` | Live typed values SDK |
| E2E | `e2e/` | End-to-end tests (depends on SDKs) |

Modules are independent — `go install .../cmd/ccs@latest` pulls only CLI deps, not server internals.

## Running Locally

### Start dependencies

```bash
docker compose up -d postgres redis
```

### Run migrations

```bash
make migrate
```

### Run the service

```bash
make build
DB_WRITE_URL="postgres://centralconfig:localdev@localhost:5432/centralconfig?sslmode=disable" \
DB_READ_URL="postgres://centralconfig:localdev@localhost:5432/centralconfig?sslmode=disable" \
REDIS_URL="redis://localhost:6379" \
./bin/central-config-service
```

No JWT setup needed — the service defaults to metadata-based auth. Pass `x-subject` in gRPC metadata.

### Run the full stack (service in Docker)

```bash
docker compose up -d --wait service
```

## Testing

### Unit Tests

```bash
make test
```

Unit tests use testify for assertions and mock interfaces for dependencies (storage, cache, pubsub). ~1.2 seconds for the full suite.

### End-to-End Tests

```bash
make e2e
```

Starts PostgreSQL, Redis, and the service via Docker Compose, runs e2e tests against the running service, and tears everything down.

E2e tests are split by domain:
- `schema_test.go` — schema lifecycle, YAML export/import
- `config_test.go` — full config flow, versioning, locks, audit
- `typed_test.go` — typed values, null handling
- `validation_test.go` — constraint enforcement, strict mode
- `stream_test.go` — gRPC subscription streaming
- `errors_test.go` — error cases and sentinel errors

### Benchmarks

```bash
# Unit benchmarks
make bench

# E2E benchmarks (against docker stack)
make bench-e2e
```

## Code Style

- Follow standard Go conventions
- Run `make lint` before submitting — it runs golangci-lint with the project's configuration
- Proto files are linted with `buf lint` and checked for breaking changes with `buf breaking`
- All public SDK methods must have godoc comments (docs will be generated)

## Submitting Changes

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes following the specs-first workflow
4. Ensure `make all` passes (generate, lint, test, build)
5. Open a pull request against `main`

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
