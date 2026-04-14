# Wishlist — Future Work

**Status:** Backlog

---

Items roughly prioritized. Not committed to — this is a living list of ideas.

## Infrastructure

- [ ] **Usage stats recording** — async batched read tracking (deferred from AuditService)
- [ ] **Internal coverage** — cache/pubsub need Redis mocks to reach 65%+
- [ ] **Cursor-based pagination** — ListSchemas/ListTenants/ListVersions (TODO in schema/service.go)

## Contrib Integrations

- [ ] **contrib/viper** — viper remote provider backed by configclient
- [ ] **contrib/koanf** — koanf provider for the Koanf config library
- [ ] **contrib/envconfig** — adapter that populates struct fields from config values

## SDK Enhancements

- [ ] **configwatcher write-through** — allow writes via watcher that optimistically update local values
- [ ] **configwatcher field groups** — register a struct and auto-map fields by tag
- [ ] **SDK code generation** — generate typed config structs from schema definitions

## Service Features

- [ ] **Webhook notifications** — HTTP callbacks on config changes (alternative to gRPC streaming)
- [ ] **Config diffing API** — server-side diff between two config versions
- [ ] **Schema migration assistant** — tooling to help migrate config values when schema changes
- [ ] **Multi-environment promotion** — promote config from dev → staging → prod with approval workflow
- [ ] **Config templates** — default config values defined at schema level, applied on tenant creation

## CLI

- [ ] **Homebrew formula** — tap repo with goreleaser-generated formula

## Observability

- [ ] **Grafana dashboard template** — pre-built dashboard for OTel metrics
- [ ] **Alerting rules** — Prometheus rules for DB pool exhaustion, high error rates

## Documentation

- [ ] **Architecture decision records (ADRs)** — formalize key decisions into ADR format
- [ ] **Social preview** — repo card image for link sharing
- [ ] **Blog post / announcement** — introduce OpenDecree
