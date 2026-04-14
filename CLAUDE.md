# OpenDecree — Claude Context

## Overview

Schema-driven business configuration management service. Multi-tenant, gRPC API, PostgreSQL + Redis.

## Tech Stack

| Concern | Tool | Version |
|---------|------|---------|
| Language | Go | 1.24 |
| API | gRPC (Protocol Buffers) | — |
| Proto tooling | buf (local plugins) | v1.66.1 |
| DB | PostgreSQL | 17 |
| DB queries | sqlc | v1.30.0 |
| DB migrations | goose | v3.27.0 |
| Cache + pub/sub | Redis | 7 |
| Auth | JWT (JWKS validation) | — |
| Testing | testify | — |
| Deployment | Kubernetes (Helm) | — |
| Observability | OpenTelemetry | — |

## Development

### Prerequisites

Go 1.24+, Docker, Make. All generators run in Docker.

### Key Commands

```bash
make generate    # buf + sqlc code generation (Docker)
make test        # go test ./...
make lint        # golangci-lint + buf lint + buf breaking
make build       # go build → bin/decree
make e2e         # docker compose → migrate → e2e tests → teardown
make clean       # remove bin/ and generated code
```

### Specs-First Workflow

1. Edit `.proto` files in `proto/centralconfig/v1/` or `.sql` files in `db/queries/`
2. Run `make generate`
3. Implement/update Go code in `internal/`
4. Run `make test` and `make lint`
5. Commit — generated files are checked into git

### Generated Code

- Proto → `api/centralconfig/v1/*.pb.go` (committed)
- sqlc → `internal/storage/dbstore/*.gen.go` (committed)
- Both are marked in `.gitattributes` as `linguist-generated`

## Project Structure

```
go.work              # Go workspace (service + api + sdk)
cmd/server/          # Entry point
proto/               # Protobuf definitions (API source of truth)
api/                 # Generated proto code (own module, lightweight deps)
sdk/                 # Client SDK (own module, depends on api/)
db/queries/          # SQL queries (DB source of truth)
db/migrations/       # goose migrations
internal/
├── server/          # gRPC server setup, interceptors
├── schema/          # SchemaService implementation
├── config/          # ConfigService implementation
├── audit/           # AuditService implementation
├── auth/            # JWT validation
├── validation/      # Field validation
├── pubsub/          # Change propagation (Redis impl behind interface)
├── cache/           # Config cache (Redis impl behind interface)
└── storage/         # DB layer + sqlc generated code
build/               # Dockerfiles (service + tools)
deploy/helm/         # Helm chart
e2e/                 # End-to-end tests
```

## Architecture

Single Go binary, three gRPC services (SchemaService, ConfigService, AuditService). Services are selectively enabled via `ENABLE_SERVICES` env var for deployment flexibility.

## Project Management

- **Milestones** on GitHub track efforts (e.g. "Admin GUI", "Security Review")
- **`.agents/context/`** holds design context for AI agents: completed effort archive, active design briefs
- **`docs/development/checklists.md`** has standard dev workflow checklists (commit, PR, release)
- **`docs/development/threat-model.md`** has the security threat model
- **GitHub Issues** are the single source of truth for tasks — no separate effort tracking files

## Conventions

- Vanilla dependencies only — standard, widely-adopted tools
- External dependencies (Redis) behind Go interfaces for replaceability
- All tool versions pinned
- buf plugins run locally, not remote
- sqlc generated files use `.gen.go` suffix
- Apache 2.0 license
