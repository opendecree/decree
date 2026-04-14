# OpenDecree — Implementation

**Status:** In Progress (Helm remaining)
**Started:** 2025-03-25

---

## Remaining

- [ ] Helm chart — Kubernetes deployment
- [ ] Usage stats recording (async, batched) — deferred

## Completed

All core services (Schema, Config, Audit), auth, validation, OTel, SDKs, CLI, benchmarks, docs, CI — see `completed.md` for details.

## Key Decisions

1. **Go 1.24** — pinned across all modules
2. **JWT auth is opt-in** — metadata headers default, same Claims in context either way
3. **Atomic config writes** — RunInTx wraps version + values + audit. Side effects after commit.
4. **Slug names** — enforced on schema/tenant names (`[a-z0-9]([a-z0-9-]*[a-z0-9])?`, 1-63 chars)
5. **TypedValue oneof** — native proto types, DB stores strings, SDK returns Go types
6. **Strict mode** — writes to unknown fields rejected, constraint/type compatibility validated at schema creation
7. **xxHash checksums** — stored in DB column, computed on write, zero cost on read
8. **Config import modes** — merge (default), replace, defaults
9. **Single version number** — server + CLI tagged together, injected via ldflags
