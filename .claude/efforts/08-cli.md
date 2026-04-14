# CLI Tool

**Status:** Phase 1+2+3 Complete

---

## Completed

31 commands across 6 groups (schema, tenant, config, watch, lock, audit) + 5 power tools (diff, docgen, validate, seed, dump) + version + gen-docs. Own Go module. 17 unit tests for CLI structure + 5 unit test files for tools packages.

### Phase 3 — Power Tools (as reusable Go packages)

New module: `sdk/tools` with 5 sub-packages, each independently importable:

- [x] `decree diff <tenant-id> <v1> <v2>` — config version diffing (`sdk/tools/diff`)
- [x] `decree docgen [schema-id] [--file]` — schema → markdown docs (`sdk/tools/docgen`)
- [x] `decree validate --schema --config [--strict]` — offline YAML validation (`sdk/tools/validate`)
- [x] `decree seed <file> [--auto-publish]` — bootstrap from single YAML (`sdk/tools/seed`)
- [x] `decree dump <tenant-id> [--version] [--no-locks]` — full tenant backup (`sdk/tools/dump`)

Key design: offline tools (diff, docgen, validate) have zero gRPC/proto deps. Online tools (seed, dump) use adminclient. Seed/dump share a YAML format — dump output feeds directly into seed.

## Phase 4 — Polish (wishlist)

- [ ] Shell completion (bash, zsh, fish — cobra built-in)
- [ ] Man page generation
- [ ] Homebrew formula / goreleaser