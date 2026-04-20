# Seed Decoupling — Design Brief

**Status:** Discovery accepted. Implementation tracked in follow-up issue.
**Related:** #137 (discovery), Schema Spec v0.1.0 milestone
**Last updated:** 2026-04-20

## Problem

`decree seed <file>` requires a combined envelope (`spec_version + schema + tenant + config + locks`). The envelope couples schema and config into one artifact, but their deployment lifecycles are different:

- **Schema** ships with the application, source-controlled next to code, published once per release.
- **Config** ships per deployment — each tenant (`org1`, `org2`, …) applies its own config against the already-published schema at deploy time.

Forcing both into one seed file leaks schema ownership into every deploy pipeline and invites drift.

The underlying API (`ImportSchema` → `CreateTenant` → `ImportConfig`) already supports 1 schema → N tenants → N configs. Only seed's CLI + seed-package orchestration couples them.

## Decision

**Option B — content-typed seed files.** One CLI entry point (`decree seed <file>`), dispatch by which top-level sections the file contains.

Three valid shapes:

### Schema-only

```yaml
# app-deploy pipeline: `decree seed schema.seed.yaml`
spec_version: v1
schema:
  name: payments
  fields: { ... }
```

Runs `ImportSchema` (with `--auto-publish` if set). No tenant, no config.

### Config-only

```yaml
# org-deploy pipeline: `decree seed org1.config.seed.yaml`
spec_version: v1
tenant:
  name: org1
  schema: payments             # required — which schema to bind to
  # schema_version omitted     # → defaults to latest published
config:
  values:
    payments.enabled: { value: true }
locks:
  - field_path: payments.currency
    locked_values: [USD]
```

Runs `CreateTenant` (reusing existing) → `ImportConfig` → `LockField`. Never touches schema.

### Combined (unchanged)

Current envelope still works — schema + tenant + config + locks in one file. No breaking change.

## Rationale (B over A, C, D)

| Option | Why rejected |
|--------|--------------|
| A — split subcommands (`decree seed schema`, `decree seed config`) | Two CLI verbs without a real win. File content already tells you the lifecycle; no reason to make the user repeat it on the command line. Breaks muscle memory for combined form. |
| C — directory convention (`decree seed ./config/`) | Wrong fit for the split-deploy case. Schema and config don't live in the same directory at deploy time (one ships with the app binary, the other with the tenant's deploy pipeline). |
| D — manifest / multi-doc YAML | Re-couples the lifecycles into one artifact. Exact opposite of the goal. |

B is the minimal change: three `nil`/`""` checks dispatch to the right RPC sequence. Existing combined files stay valid. Each pipeline ships the file it owns.

## Schema version defaulting

**Config-only files may omit `tenant.schema_version`.** When omitted, seed resolves the **latest published version** of the named schema.

YAML representation: **omit the field**, not a sentinel like `"latest"` or `0`. Rationale: cleanest for YAML schema validation (field is simply absent, not a magic value), and Go code checks `*int32 == nil`.

This yields the "deploy this config against whatever schema is currently live" workflow that matches how org-deploy pipelines think.

### Resolution

Implemented client-side in the seed package (not server-side RPC). Keeps the proto surface unchanged.

Algorithm:
1. `ListSchemas(ctx)` → find entry where `Name == tenant.schema`
2. If `schema.Published == true` → use `schema.ID`, `schema.Version`
3. Otherwise → iterate `GetSchemaVersion(ctx, schema.ID, v)` for `v = schema.Version - 1` downward until `Published == true`
4. If no published version exists → error: `"no published version of schema %q found"`

**Known gap:** there is no `ListSchemaVersions` / `GetLatestPublishedVersion` RPC today. Implementation issue should decide whether to (a) iterate as above or (b) add a new server RPC. (a) is simpler for MVP; (b) is cheaper if draft-after-published becomes common.

## Dispatch rules (seed package)

After `ParseFile`, inspect the parsed `File`:

| Schema present | Tenant present | Config present | Mode | Action |
|---|---|---|---|---|
| ✓ | ✗ | ✗ | schema-only | `ImportSchema` (+ auto-publish) |
| ✗ | ✓ | ✓ | config-only | Resolve schema → `CreateTenant` (reuse if exists) → `ImportConfig` → `LockField`* |
| ✓ | ✓ | ✓ | combined | Current behavior — all three |
| ✓ | ✓ | ✗ | schema + tenant | `ImportSchema` → `CreateTenant`. Useful for bootstrap. |
| ✓ | ✗ | ✓ | invalid | Error: `"config section requires tenant section"` |
| ✗ | ✓ | ✗ | tenant-only | `CreateTenant` (reuse if exists). Edge but valid. |
| ✗ | ✗ | ✓ | invalid | Error: `"config section requires tenant section"` |
| ✗ | ✗ | ✗ | invalid | Error: `"at least one of schema, tenant, or config must be present"` |

*Locks always bind to the tenant, so they're only valid in modes that create/resolve a tenant.

## Validation changes (`ParseFile`)

Currently forces all three sections. New rules:

- `spec_version == "v1"` still required in all modes
- If `schema:` present → `schema.name` and non-empty `schema.fields` required
- If `tenant:` present → `tenant.name` required; `tenant.schema` required in config-only mode (schema reference); `tenant.schema_version` optional (nil = latest published)
- If `config:` present → `tenant:` also required
- At least one of `schema:`, `tenant:`, `config:` must be non-empty

## Idempotency

Seeding the same content twice must be a no-op — no new schema version, no new config version, no error.

Current state:
- **Schema:** `ImportSchema` returns `ErrAlreadyExists` when fields are identical; `seed.Run()` handles this and reuses the existing version. ✓
- **Config:** server already returns `codes.AlreadyExists` with `"no changes to apply"` when merge-mode filtering finds nothing to change (internal/config/service.go:842). But the adminclient doesn't translate this to `ErrAlreadyExists`, and `seed.Run()` doesn't handle it. A no-op config re-seed surfaces as a hard error today. ✗

Fix alongside the decoupling work (affects all modes, not just the new ones):
- `sdk/adminclient/config.go` — `ImportConfig` must map grpc `codes.AlreadyExists` → `adminclient.ErrAlreadyExists`
- `sdk/tools/seed/seed.go` — `Run()` must treat `ErrAlreadyExists` from `ImportConfig` as a successful skip (set `ConfigImported=false`, preserve the existing `ConfigVersion` from the latest version lookup)
- **Locks:** `LockField` is inherently idempotent (re-locking to the same values is fine). Confirm in test.

## Compatibility

- **spec_version:** no bump. Additive at parser/orchestrator layer.
- **Proto / server API:** no changes. All decoupled RPCs exist today.
- **Combined envelope:** still valid, no deprecation planned.
- **Adminclient:** one new method, `GetLatestPublishedSchemaVersion(ctx, schemaName)` (wraps ListSchemas + optional GetSchemaVersion iteration).

## Impact summary (for follow-up issue)

- `sdk/tools/seed/seed.go` — make Schema optional; add `TenantDef.Schema` (name ref) and `TenantDef.SchemaVersion *int32`; rewrite `ParseFile` validation; rewrite `Run()` to dispatch on mode
- `sdk/tools/seed/seed_test.go` — coverage for all modes + latest-version resolution + error paths
- `sdk/adminclient/schema.go` — add `GetLatestPublishedSchemaVersion`
- `sdk/adminclient/types.go` — no changes
- `cmd/decree/seed.go` — no changes (CLI is transparent; dispatch is in seed package)
- `examples/` — add `schema.seed.yaml` and `org1.config.seed.yaml` siblings to the combined `seed.yaml`
- `docs/usecases/config-as-code.md` — show decoupled schema/config seed workflow alongside the current `schema import` / `config import` primitives
- `docs/cli/decree_seed.md` — document the three modes

No changes needed in:
- `decree-python`, `decree-typescript` (no seed tooling there; examples can optionally mirror)
- `demos/` (existing quickstart uses combined form; no forced migration)
- Proto / server (decoupled RPCs already exist)

## Open questions (for implementation)

1. **Tenant-schema mismatch:** if the tenant already exists on schema A and the config-only file names schema B, error or re-bind? **Default:** error, with message naming both schemas. Re-binding is a separate feature.
2. **`tenant.schema` field name:** keep as `schema` (short) or rename to `schema_name` for clarity? Combined form currently infers schema from the co-located `schema:` section, so this field is new. **Default:** `schema:` (matches how users think — "tenant points at schema X").
3. **Latest-published resolution cost:** if the schema has N draft versions stacked on top of the latest published one, we'd call `GetSchemaVersion` N times. MVP: iterate. If this becomes hot, add a `ListSchemaVersions` RPC.

## Non-goals

- Splitting `decree seed` into subcommands (option A). Rejected above.
- Directory mode (`decree seed ./config/`). Separate feature if wanted later.
- Changing the combined envelope. Stays as-is.
- Introducing manifests or multi-doc YAML. Rejected.
