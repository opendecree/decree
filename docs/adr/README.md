# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for OpenDecree. ADRs document significant architectural choices, the context that motivated them, and their consequences.

## Format

Each ADR follows this structure:

- **Context** — the situation or problem that prompted the decision
- **Decision** — what was decided
- **Consequences** — the results of the decision, both positive and negative

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [ADR-001](ADR-001-grpc-over-rest.md) | gRPC over REST | Accepted |
| [ADR-002](ADR-002-specs-first-workflow.md) | Specs-First Workflow | Accepted |
| [ADR-003](ADR-003-postgresql-redis-architecture.md) | PostgreSQL + Redis Architecture | Accepted |
| [ADR-004](ADR-004-metadata-headers-first-auth.md) | Metadata-Headers-First Authentication | Accepted |
| [ADR-005](ADR-005-multi-tenant-rls.md) | Multi-Tenant Isolation via Row-Level Security | Accepted |
| [ADR-006](ADR-006-go-module-split.md) | Go Module Split (8 Modules) | Accepted |

## Adding a new ADR

1. Create a file named `ADR-NNN-short-title.md` (increment the number)
2. Use the template below
3. Add a row to the index above

```markdown
# ADR-NNN: Title

**Date:** YYYY-MM-DD
**Status:** Accepted
**Deciders:** OpenDecree maintainers

## Context

[What situation or problem prompted this decision]

## Decision

[What was decided]

## Consequences

[What are the results of this decision — good and bad]
```
