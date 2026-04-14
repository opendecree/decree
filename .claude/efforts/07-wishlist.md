# Wishlist — Future Work

**Status:** Backlog

---

Items roughly prioritized. Not committed to — this is a living list of ideas.

## Infrastructure

- [ ] **Helm chart** — Kubernetes deployment with configurable replicas, resource limits, env vars
- [ ] **CI (GitHub Actions)** — lint, test, build, e2e on PR; image push on merge
- [ ] **Usage stats recording** — async batched read tracking (deferred from AuditService)

## Contrib Integrations

- [ ] **contrib/viper** — viper remote provider backed by configclient (read-only config source)
- [ ] **contrib/koanf** — koanf provider for the Koanf config library
- [ ] **contrib/envconfig** — adapter that populates struct fields from config values

## SDK Enhancements

- [ ] **Retry/backoff on configclient** — automatic retry with backoff for transient gRPC errors
- [ ] **configwatcher write-through** — allow writes via watcher that optimistically update local values
- [ ] **configwatcher field groups** — register a struct and auto-map fields by tag
- [ ] **SDK code generation** — generate typed config structs from schema definitions

## Service Features

- [ ] **Field validation on write** — validate values against schema constraints server-side (currently schema defines constraints but ConfigService doesn't enforce them)
- [ ] **Webhook notifications** — HTTP webhook callbacks on config changes (alternative to gRPC streaming)
- [ ] **Config diffing** — API to diff two config versions and return changed fields
- [ ] **Schema migration assistant** — tooling to help migrate config values when schema changes
- [ ] **Multi-environment promotion** — promote config from dev → staging → prod with approval workflow
- [ ] **Config templates** — default config values defined at schema level, applied on tenant creation

## Observability

- [ ] **Grafana dashboard template** — pre-built dashboard for the OTel metrics
- [ ] **Alerting rules** — Prometheus alerting rules for DB pool exhaustion, high error rates

## Documentation

- [ ] **godoc site** — hosted godoc for all SDK packages
- [ ] **API reference** — generated from proto comments
- [ ] **Getting started guide** — end-to-end tutorial: schema → tenant → config → SDK usage
- [ ] **Architecture decision records (ADRs)** — formalize key decisions from efforts into ADR format
