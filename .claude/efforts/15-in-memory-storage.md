# In-Memory Storage Implementation

**Status:** Complete
**Started:** 2026-03-30

---

## Goal

Implement in-memory backends for all pluggable interfaces. Proves the storage abstraction works, enables fast tests without Docker, and provides a lightweight deployment option.

## Components (all in single PR)

- [x] `internal/cache/memory.go` — map + TTL + mutex, implements ConfigCache
- [x] `internal/pubsub/memory.go` — channel-based fan-out, implements Publisher + Subscriber
- [x] `internal/audit/store_memory.go` — slice-based, implements audit.Store (5 methods)
- [x] `internal/schema/store_memory.go` — map-based, implements schema.Store (~20 methods)
- [x] `internal/config/store_memory.go` — map-based with version chain, implements config.Store (~15 methods)
- [ ] Wire up in cmd/server (STORAGE_BACKEND=memory) — future PR

## Design

- Each in-memory impl lives alongside the PG impl in the same package
- `sync.RWMutex` for thread safety
- No external dependencies (vanilla Go only)
- `domain.ErrNotFound` for missing entities
- Auto-incrementing IDs (`mem-00000001`)
- Config store: RunInTx serialized via mutex

## Coverage Impact

Internal coverage: 50.2% → 58.2% (+8%)
