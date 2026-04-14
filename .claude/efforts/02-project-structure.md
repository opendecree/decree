# Central Config Service — Project Structure & Dev Cycle

**Status:** Complete
**Started:** 2025-03-25

---

## 1. Development Cycle

### Flow

```
modify → generate → test → lint → build → deploy → e2e
```

### Principles

- **Generators run in Docker** — buf, sqlc, goose. No local tool installation beyond Go + Docker + Make.
- **Go runs locally** — test, build, lint benefit from local caching and IDE integration.
- **Make is the interface** — every dev action is a Make target.
- **Docker Compose for local e2e** — one command spins up the full stack.

### Makefile Targets

| Target | What it does | Runs in |
|--------|-------------|---------|
| `make generate` | buf generate + sqlc generate (single container) | Docker |
| `make test` | go test ./... | Local |
| `make lint` | golangci-lint + buf lint + buf breaking | Local (lint), Docker (buf) |
| `make build` | go build -o bin/central-config-service ./cmd/server | Local |
| `make image` | docker build the service image | Docker |
| `make deploy` | helm install/upgrade to k8s | Local (helm CLI) |
| `make e2e` | docker compose up → migrate → run e2e tests → docker compose down | Docker |
| `make migrate` | run goose migrations | Docker |
| `make clean` | remove bin/, generated code | Local |
| `make all` | generate → lint → test → build | Mixed |

### Build Caching

- **Tools image sentinel**: `.tools-image-built` file tracks when the tools Docker image was last built. The image only rebuilds when `build/Dockerfile.tools` changes — subsequent `make generate` calls skip `docker build` entirely (~0.3s vs ~7s).
- **Single container per target**: `make generate` runs both `buf generate` and `sqlc generate` in one `docker run`. `make lint-proto` runs both `buf lint` and `buf breaking` in one container.

---

## 2. Directory Layout

```
central-config-service/
│
├── cmd/
│   └── server/
│       └── main.go                  # Entry point, flag parsing, service wiring
│
├── proto/                           # Protobuf definitions (source of truth)
│   └── centralconfig/
│       └── v1/
│           ├── schema_service.proto
│           ├── config_service.proto
│           ├── audit_service.proto
│           └── types.proto          # Shared message types
│
├── gen/                             # Generated code (committed, marked in .gitattributes)
│   └── go/
│       └── centralconfig/
│           └── v1/
│               ├── *.pb.go
│               └── *_grpc.pb.go
│
├── internal/                        # Private application code
│   ├── server/                      # gRPC server setup, interceptors
│   │   ├── server.go
│   │   └── interceptors.go
│   │
│   ├── schema/                      # SchemaService implementation
│   │   ├── service.go
│   │   ├── store.go                 # DB queries interface
│   │   └── service_test.go
│   │
│   ├── config/                      # ConfigService implementation
│   │   ├── service.go
│   │   ├── store.go
│   │   ├── cache.go
│   │   ├── subscriber.go
│   │   └── service_test.go
│   │
│   ├── audit/                       # AuditService implementation
│   │   ├── service.go
│   │   ├── store.go
│   │   └── service_test.go
│   │
│   ├── auth/                        # JWT validation, role extraction
│   │   ├── jwt.go
│   │   └── jwt_test.go
│   │
│   ├── validation/                  # Field validation logic
│   │   ├── validator.go
│   │   └── validator_test.go
│   │
│   ├── pubsub/                      # Change propagation abstraction
│   │   ├── pubsub.go               # Interface
│   │   └── redis.go                # Redis implementation
│   │
│   ├── cache/                       # Config cache abstraction
│   │   ├── cache.go                # Interface
│   │   └── redis.go                # Redis implementation
│   │
│   └── storage/                     # Database layer
│       ├── postgres.go              # Connection setup, read/write routing
│       └── dbstore/                 # sqlc generated code (committed, .gen.go suffix)
│           ├── db.gen.go
│           ├── models.gen.go
│           └── *.sql.gen.go
│
├── db/                              # sqlc configuration and SQL source
│   ├── sqlc.yaml
│   ├── queries/                     # Hand-written SQL queries
│   │   ├── schemas.sql
│   │   ├── config.sql
│   │   ├── tenants.sql
│   │   └── audit.sql
│   └── migrations/                  # goose migration SQL files
│       ├── 001_initial_schema.sql
│       └── ...
│
├── e2e/                             # End-to-end tests
│   ├── e2e_test.go
│   └── ...
│
├── deploy/                          # Deployment manifests
│   └── helm/
│       └── central-config-service/
│           ├── Chart.yaml
│           ├── values.yaml
│           └── templates/
│               ├── deployment.yaml
│               ├── service.yaml
│               ├── configmap.yaml
│               └── ...
│
├── build/                           # Build-related files
│   ├── Dockerfile                   # Service image
│   └── Dockerfile.tools             # Generator tools (buf, sqlc, goose)
│
├── buf.yaml                         # Buf configuration
├── buf.gen.yaml                     # Buf code generation config
├── docker-compose.yml               # Local dev: PG + Redis + service
├── Makefile
├── go.mod
├── go.sum
├── .golangci.yml                    # Linter config
├── .gitignore
├── LICENSE
└── README.md
```

### Key Decisions

**`proto/` in this repo** — protos live alongside the code. If we ever need a shared proto repo, we extract later.

**`gen/` is committed** — generated code is checked into the repo so `go build` works without running generators. `.gitattributes` marks generated files so GitHub collapses them in PR diffs. sqlc files use `.gen.go` suffix for clarity.

**`internal/` for all app code** — nothing is importable by external packages. If we add a Go client SDK later, it gets its own top-level directory and go.mod (workspace).

**`db/` vs `internal/storage/`** — SQL source files (queries, migrations) live in `db/` as the spec source. sqlc generates code into `internal/storage/dbstore/`. Migrations live in `db/migrations/` (single source).

**Service packages** (`internal/schema/`, `internal/config/`, `internal/audit/`) — each owns its gRPC service implementation and defines a store interface that the storage layer satisfies.

---

## 3. Docker Setup

### Dockerfile.tools

Single image with all generator tools (pinned versions):
- buf v1.66.1
- sqlc v1.30.0
- goose v3.27.0
- protoc-gen-go v1.36.11 + protoc-gen-go-grpc v1.6.1

Used by `make generate` and `make migrate`.

### Dockerfile (service)

Multi-stage build:
1. **Build stage** — Go builder, compiles the binary
2. **Runtime stage** — minimal image (distroless or alpine), just the binary

### docker-compose.yml

```yaml
services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: centralconfig
      POSTGRES_USER: centralconfig
      POSTGRES_PASSWORD: localdev
    ports:
      - "5432:5432"

  redis:
    image: redis:7
    ports:
      - "6379:6379"

  service:
    build:
      context: .
      dockerfile: build/Dockerfile
    depends_on:
      - postgres
      - redis
    environment:
      DB_WRITE_URL: postgres://centralconfig:localdev@postgres:5432/centralconfig?sslmode=disable
      DB_READ_URL: postgres://centralconfig:localdev@postgres:5432/centralconfig?sslmode=disable
      REDIS_URL: redis://redis:6379
      ENABLE_SERVICES: schema,config,audit
    ports:
      - "9090:9090"
```

---

## 4. Configuration

The service itself is configured via environment variables (dogfooding is tempting but adds circular complexity):

| Variable | Description | Default |
|----------|------------|---------|
| `GRPC_PORT` | gRPC listen port | 9090 |
| `DB_WRITE_URL` | PostgreSQL primary connection string | required |
| `DB_READ_URL` | PostgreSQL read replica connection string | falls back to DB_WRITE_URL |
| `REDIS_URL` | Redis connection string | required |
| `ENABLE_SERVICES` | Comma-separated: schema,config,audit | all |
| `JWT_ISSUER` | Expected JWT issuer | optional |
| `JWT_JWKS_URL` | JWKS endpoint for JWT validation | required |
| `LOG_LEVEL` | debug, info, warn, error | info |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry collector endpoint | optional |

---

## 5. Open Questions

- [ ] Helm chart details — resource limits, HPA config, etc.

### Resolved

- Go version: 1.24
- golangci-lint: standard config with additional linters (bodyclose, errorlint, gocritic, gofumpt, etc.)
- License: Apache 2.0
- Generated files: committed to repo with `.gitattributes` for PR diff collapsing
- Tool versions: all pinned in Dockerfile.tools
- buf plugins: local (not remote) for offline reproducibility
