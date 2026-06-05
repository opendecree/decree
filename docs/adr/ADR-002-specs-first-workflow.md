# ADR-002: Specs-First Workflow

**Date:** 2026-06-05
**Status:** Accepted
**Deciders:** OpenDecree maintainers

## Context

OpenDecree has two distinct schema layers that must remain consistent:

1. **API contracts** — defined in `.proto` files, which drive gRPC server interfaces and all SDK clients
2. **Database contracts** — defined in `.sql` query files, which drive the Go database access layer via `sqlc`

Without a clear source-of-truth policy, there is a risk that hand-written Go code diverges from the API or DB schema, especially as the project evolves across multiple contributors and multiple language SDKs. A "code-first" approach would mean maintaining three or more representations of the same concept in sync by hand.

## Decision

Proto files (`proto/centralconfig/v1/`) and SQL query files (`db/queries/`) are the canonical sources of truth. Go implementation code is written after — and in response to — changes in those files.

The standard workflow is:

1. Edit `.proto` or `.sql` files
2. Run `make generate` (runs `buf generate` and `sqlc generate` in Docker)
3. Implement or update Go business logic in `internal/`
4. Run `make test` and `make lint`
5. Commit — generated files (`*.pb.go`, `*.gen.go`) are checked into git

Generated files are marked as `linguist-generated` in `.gitattributes` so they are excluded from language statistics and diff noise on GitHub.

## Consequences

**Positive:**

- Single source of truth for both the API surface and the DB access layer
- Consistency is mechanically enforced — divergence between spec and implementation is a compile error
- Multi-language SDK clients are always in sync with the server API without manual effort
- `buf breaking` catches accidental API regressions at lint time
- `sqlc` type-checks SQL at generation time, catching query errors before runtime

**Negative:**

- Extra codegen step is required before implementing any change — developers must run `make generate` before editing business logic
- Generated files are committed to git, which increases diff size and requires discipline to avoid manual edits to generated files
- Mistakes in a proto or SQL file can break generation for all downstream modules until fixed
- Local development requires Docker to run `buf` and `sqlc` generators
