# ADR-003: PostgreSQL + Redis Architecture

**Date:** 2026-06-05
**Status:** Accepted
**Deciders:** OpenDecree maintainers

## Context

OpenDecree needs to:

1. Durably store configuration values, schema definitions, and an immutable audit log across multiple tenants
2. Serve config reads with low latency — config lookups are on the hot path for every service that depends on this system
3. Propagate configuration changes in real time to connected clients (config watcher subscriptions)

A single data store would force tradeoffs: a relational database handles durable storage and ACID guarantees well but is not the right tool for low-latency cache reads or fan-out pub/sub; an in-memory store handles cache and messaging well but lacks the durability and query capabilities needed for the config history and audit log.

## Decision

Use **PostgreSQL 17** as the primary durable store and **Redis 7** as the cache and pub/sub layer.

- PostgreSQL stores all config values (versioned), schema definitions, tenant records, and the audit log. Row-Level Security enforces tenant isolation at the database layer (see ADR-005).
- Redis caches resolved config values for fast reads. On a cache miss, the server fetches from PostgreSQL and repopulates the cache.
- Redis pub/sub propagates change notifications to all server instances when a config value is written, so connected config-watcher clients receive updates promptly across a horizontally scaled deployment.

Both dependencies are placed behind Go interfaces (`cache.Cache`, `pubsub.PubSub`) so they can be swapped or mocked in tests without touching business logic.

## Consequences

**Positive:**

- Proven, widely-adopted technologies with strong operational tooling and hosting options
- Full ACID guarantees from PostgreSQL for config mutations and audit entries
- Optimistic read path: most config reads are served from Redis without hitting the database
- Real-time change propagation across server replicas with minimal coupling
- Interfaces-behind-abstraction pattern keeps the business logic testable and the dependencies replaceable

**Negative:**

- Two external infrastructure dependencies increase operational complexity compared to a single-store solution
- Redis is a single point of failure for pub/sub — if Redis is unavailable, change notifications are not delivered (reads can still be served from PostgreSQL, but watchers will not see updates until Redis recovers)
- Cache invalidation logic must be kept in sync with write paths; a missed invalidation leads to stale reads until TTL expiry
- Local development and CI require both PostgreSQL and Redis to be running (addressed via Docker Compose)
