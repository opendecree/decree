# ADR-001: gRPC over REST

**Date:** 2026-06-05
**Status:** Accepted
**Deciders:** OpenDecree maintainers

## Context

OpenDecree is a multi-tenant configuration service that needs to support client SDKs in multiple languages (Go, Python, TypeScript). The API must support:

- Strongly typed contracts shared across the server and all SDK clients
- Efficient binary serialization for high-throughput config reads
- Streaming for real-time config change notifications (config watcher pattern)
- Automatic client code generation to keep SDKs in sync with the server

REST/JSON APIs require manual schema maintenance (OpenAPI), are weakly typed at the transport layer, and lack native streaming. HTTP/2 multiplexing and binary framing make gRPC a better fit for an SDK-centric, performance-sensitive service.

## Decision

Use Protocol Buffers (proto3) as the API schema language and gRPC as the primary transport. Proto files in `proto/centralconfig/v1/` are the single source of truth for the API contract. Generated code is committed to `api/centralconfig/v1/`.

`buf` is used for linting, breaking-change detection, and code generation (running plugins locally via Docker, not remote registries).

## Consequences

**Positive:**

- Strong typing enforced at compile time across all languages
- Automatic client stub generation via `buf generate` — SDKs are always in sync
- Native bidirectional streaming for config watch notifications
- Compact binary encoding (Protocol Buffers) reduces latency and bandwidth vs. JSON
- `buf lint` and `buf breaking` catch API contract violations before they ship

**Negative:**

- Browser clients cannot call gRPC directly; grpc-web or a transcoding gateway is required
- Not curl-friendly — human debugging requires tooling such as grpcurl or a gRPC reflection client
- Proto schema changes must follow strict backward-compatibility rules (or be coordinated with a breaking migration during alpha)
- Adds `buf` and proto toolchain to the developer setup (mitigated by Docker-based generation)
