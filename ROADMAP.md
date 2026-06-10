# OpenDecree Roadmap

> **Alpha software.** APIs, proto definitions, configuration formats, and behavior may change between versions. Not recommended for production use yet.

This roadmap is the honest counterpart to that warning: what you can already build on, what is still moving, and the milestones between here and a stable **1.0**. It stays high-level on purpose — milestones, not dated promises — so it remains accurate as work lands.

For live status, see the [GitHub milestones](https://github.com/opendecree/decree/milestones) and the [project board](https://github.com/orgs/opendecree/projects/1). Raw feature ideas live in [Discussions → Ideas](https://github.com/opendecree/decree/discussions/categories/ideas).

## Where we are

Releases are in the **0.x alpha** series ([latest release](https://github.com/opendecree/decree/releases/latest)). The core service and its data model have already been through a [security review](https://github.com/opendecree/decree/issues/26), [stress testing](https://github.com/opendecree/decree/issues/27), and a beta-readiness audit. What stands between alpha and 1.0 is API stabilization, ecosystem breadth, and a public launch — **not** a core re-architecture. The foundation you build on today is what we expect to carry forward.

## Stable vs. in flux

Alpha means we reserve the right to break anything — but in practice these surfaces are not equally settled. Use this as a guide to where churn is likely:

| Surface | Maturity |
|---------|----------|
| Core type system — `TypedValue`: integer, number, string, bool, time, duration, url, json | **Stable** |
| gRPC API — SchemaService, ConfigService, AuditService | **Stable** |
| Multi-tenancy & RBAC — roles, tenant scoping, field-level locking | **Stable** |
| Audit trail & versioning — full history, rollback to any version | **Stable** |
| Storage / cache / pub-sub backends — Postgres, Redis, in-memory (behind Go interfaces) | **Stable** |
| REST/JSON gateway (grpc-gateway) | **Stable** |
| Auth — metadata headers (default), JWT/JWKS (opt-in) | **Stable** |
| SDKs — Go, Python, TypeScript | **Settling** |
| Admin GUI (decree-ui) | **Settling** |
| Observability — OpenTelemetry (opt-in) | **Settling** |
| Externally-managed fields, validation webhooks, SDK code generation | **Planned** |

What the labels mean:

- **Stable** — the shape is settled. Changes are expected to be additive, and any breaking change will be called out in the release notes.
- **Settling** — works today, but expect minor ergonomic changes. SDK surfaces track the proto; OTel span/metric names may still shift.
- **Planned** — not yet implemented. These will add proto and may shift adjacent APIs ([externally-managed fields #78](https://github.com/opendecree/decree/issues/78), [validation webhooks #77](https://github.com/opendecree/decree/issues/77), [SDK codegen #19](https://github.com/opendecree/decree/issues/19)).

## Path to 1.0

High-level milestones, roughly in order. Live status lives on the [GitHub milestones](https://github.com/opendecree/decree/milestones) page — this list deliberately avoids dates.

**Done**

- **Hardening** — usage stats, cursor pagination, security review, stress testing.
- **Admin GUI** — embedded in the server binary and released.
- **Lightweight SDKs** — testify dropped, transport decoupled from gRPC, lower Go floor.

**In progress**

- **Beta Readiness** — findings from the beta-readiness audit across server, storage, tests, and the Go SDK. Nearly complete; remaining work is documentation polish.

**Toward 1.0**

- **Go-to-Market** — README/docs rewrite (done), blog posts, and a public launch.
- **Ecosystem** — contrib integrations (viper, koanf, envconfig), SDK code generation, validation webhooks, externally-managed fields, distribution (Homebrew), observability templates (Grafana/Prometheus).
- **API stability commitment** — once the **Settling** and **Planned** surfaces above land, freeze the proto/API, document the backward-compatibility guarantees, and drop the alpha label.

**What 1.0 means:** a documented backward-compatibility promise across the gRPC/proto API and the SDK surfaces, and removal of the alpha warning. We are not there yet — but the stable surfaces above are the parts we already intend to honor.

## Backlog & ideas

Unmilestoned candidates live in the [issue backlog](https://github.com/opendecree/decree/issues); raw ideas live in [Discussions → Ideas](https://github.com/opendecree/decree/discussions/categories/ideas). See the [refinement checklist](https://github.com/opendecree/decree/discussions/55) for how an idea graduates to a tracked issue.
