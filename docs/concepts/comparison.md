# Comparison: Why not X?

> **Alpha Software** — OpenDecree is under active development. Everything below reflects the project's goals and current state honestly; features may change without notice between versions, and it is not recommended for production use yet.

If you're evaluating OpenDecree, you've probably already asked: *why not LaunchDarkly, Unleash, Flagsmith, Consul, etcd, Spring Cloud Config, AWS AppConfig — or a secrets manager?* These are good tools. Most teams already run one or more of them. This page explains where OpenDecree overlaps with each category, where it genuinely differs, and when you should reach for something else instead.

We try to be fair. The tools below are mature and well-run; OpenDecree is a young, alpha project. The goal here is an honest map, not a scoreboard.

## The wedge

OpenDecree occupies one specific intersection:

> **Typed, schema-driven, multi-tenant business configuration — open-source, polyglot, and with a UI.**

Each of those properties exists somewhere in the market. What's missing is a tool that combines *all* of them:

- **Typed** — values are `integer`, `number`, `string`, `bool`, `time`, `duration`, `url`, or `json`, not opaque strings or bytes. See [Typed Values](typed-values.md).
- **Schema-driven** — a versioned, validated schema defines what fields exist and their constraints *before* anyone sets a value. See [Schemas & Fields](schemas-and-fields.md).
- **Multi-tenant** — one schema, many tenants, each with independent values and access control. See [Tenants](tenants.md).
- **Open-source** — self-hostable, no vendor lock-in, inspectable.
- **Polyglot** — a gRPC + REST API with SDKs, not a language-specific library.
- **Has a UI** — an open-source admin GUI, not config-as-code only or a vendor-hosted console.

Any individual competitor covers some of these. None we know of covers all of them in one open-source product. That gap is the wedge.

## Feature-flag platforms

**LaunchDarkly, Unleash, Flagsmith, OpenFeature**

This is the closest category, and the most common point of confusion. Feature-flag platforms and OpenDecree both push dynamic values to running applications. The difference is *what* you're modeling.

Feature flags are **release and targeting primitives**: turn a feature on for 10% of users, gate a rollout by segment, run an A/B test. The value is usually a boolean or a small variant, and the interesting machinery is the *targeting* — rules that resolve a flag differently per user or context.

OpenDecree models **business configuration**: approval thresholds, fee structures, settlement windows, rate limits — typed, structured values that are correct or incorrect against a schema, and that a human operator sets deliberately. There's no per-user targeting engine. The interesting machinery is the *schema*: types, constraints, validation, versioning, and multi-tenancy.

| | Feature-flag platforms | OpenDecree |
|---|---|---|
| Primary object | Flag with targeting rules | Typed field in a validated schema |
| Typical value | Boolean / small variant | int, number, string, bool, time, duration, url, json |
| Per-user targeting | ✓ core feature | ✗ not the model |
| Schema + constraint validation | Limited (some remote-config validation) | ✓ enforced on every write |
| Multi-tenancy | Segments / environments | ✓ first-class tenants with access control |

**Flagsmith** deserves specific mention — it's open-source and offers "remote config" alongside flags, so it's the nearest overlap. If your remote config is a handful of untyped values and you already want flags, Flagsmith may be the simpler choice. OpenDecree differs where config itself is the product: a typed schema registry, constraint validation, versioned rollback, and multi-tenant isolation as the center of gravity rather than a feature beside flags.

**OpenFeature** is a vendor-neutral *flag-evaluation API*, not a config store — complementary rather than competing. You could plausibly use OpenFeature for flags and OpenDecree for typed business config in the same system.

**Choose a feature-flag platform when** your dominant need is release gating, gradual rollouts, or per-user/segment targeting.

## Config and coordination stores

**Consul, etcd, Spring Cloud Config, AWS AppConfig, Azure App Configuration**

These are the tools people reach for to store "some config somewhere central." They range from raw key-value stores to cloud config services.

**etcd** is a strongly-consistent KV store for *cluster state* — the backing store for Kubernetes, leader election, distributed locks. It's infrastructure plumbing. It has no notion of an application-config schema, types, tenants, or a UI, and it isn't trying to. If you need a consensus KV store, use etcd; it isn't an app-config product and OpenDecree isn't a coordination store.

**Consul** adds service discovery, health checking, and a KV store with a service mesh. Its KV is untyped, and running Consul well is an operational commitment. For *business* config you'd be building schema, typing, and validation on top yourself.

**Spring Cloud Config** is a solid, mature answer — if you live in the JVM/Spring ecosystem. It's Git-backed and Spring-native. OpenDecree is polyglot by design: the API and SDKs don't assume a language or framework.

**AWS AppConfig** and **Azure App Configuration** are the closest in *intent* — managed config services with some validation and gradual rollout. The tradeoffs are vendor lock-in and cloud coupling: you can't self-host, there's no portable schema registry, and there's no gRPC streaming. OpenDecree is open-source and self-hostable, with a typed schema registry and streaming subscriptions.

| | KV / coordination stores | Cloud config services | OpenDecree |
|---|---|---|---|
| Typed schema + validation | ✗ | Limited | ✓ |
| Multi-tenancy | ✗ | Limited | ✓ first-class |
| Self-hostable / OSS | ✓ (Consul, etcd) | ✗ vendor-locked | ✓ |
| Admin UI for config | ✗ | Vendor console | ✓ open-source GUI |
| Real-time streaming | Watch APIs | ✗ | ✓ gRPC subscriptions |
| Audit trail | ✗ / limited | Limited | ✓ full who/what/when/why |

**Choose a coordination store when** you need low-level KV, service discovery, or cluster-state consensus. **Choose a cloud config service when** you're all-in on one cloud and self-hosting isn't a requirement.

## Secrets managers — and why config is not secrets

**HashiCorp Vault, AWS Secrets Manager, Azure Key Vault, GCP Secret Manager**

This is a boundary, not a comparison. **OpenDecree is not a secrets manager, and you should not store secrets in it.**

The distinction is real and worth stating plainly:

| | Secrets | Business config |
|---|---|---|
| Examples | API keys, DB passwords, private keys, tokens | Approval thresholds, fee tables, settlement windows |
| Sensitivity | High — leakage is a breach | Operational — visible to operators by design |
| Access | Least-privilege, tightly scoped | Readable by the services and operators that use it |
| Lifecycle | Rotation, leasing, expiry, revocation | Versioning, validation, audited edits |
| Read pattern | Fetched rarely, cached carefully | Read constantly, streamed on change |

A secrets manager is built to keep sensitive values *hidden* — encrypted at rest, tightly access-controlled, rotated and leased. OpenDecree is built to make business config *visible and correct* — typed, validated, versioned, and auditable. Optimizing for one actively works against the other: OpenDecree exposes values through a UI and streams them to subscribers, which is exactly what you don't want for a private key.

**They coexist.** A typical system uses a secrets manager for credentials and OpenDecree for business parameters. Where a config value must *reference* a secret, store the reference (for example, a secret's name or path) in OpenDecree and resolve it through your secrets manager at read time — never the secret material itself.

**Choose a secrets manager when** the value is sensitive and its exposure would be a security incident. That's not OpenDecree's job.

## Honest limitations

Fair comparison cuts both ways. As of this alpha:

- **It's alpha.** APIs, the schema format, and proto definitions may break between versions without a major bump until v1.0.0. Production use is not recommended yet.
- **No per-user targeting.** If you need flag-style rollouts or A/B targeting, OpenDecree doesn't do that — pair it with a flag platform.
- **Younger ecosystem.** The tools above have years of integrations, docs, and battle-testing. OpenDecree is new.
- **You operate it.** Self-hosting the server, PostgreSQL, and Redis is your responsibility (or use a managed equivalent); there's no hosted offering.

## Summary

OpenDecree isn't trying to replace any single tool above. It fills a gap *between* them: the typed, schema-driven, multi-tenant, open-source, polyglot config layer with a UI that none of these categories covers head-on.

- Need **release gating / targeting**? Use a feature-flag platform (optionally alongside OpenDecree).
- Need **cluster state / service discovery**? Use etcd or Consul.
- Need **secrets**? Use a secrets manager — always.
- Need **typed, validated, multi-tenant business config** you can self-host, reach from any language, and edit through a UI with a full audit trail? That's the wedge OpenDecree is built for.

See the [Overview](overview.md) for the mental model, or [Getting Started](../getting-started.md) to try it.
