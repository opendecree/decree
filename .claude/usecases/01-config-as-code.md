# Use Case: Config as Code (Single Tenant, Single Schema)

**Status:** Decisions Made

---

## Scenario

A team has one service, one tenant, one schema. They want:

1. **Schema in git** — the schema YAML lives in the project repo. On each deploy/startup, the schema is synced to DECREE (only if changed). The repo is the source of truth for structure.

2. **Baseline config values in git** — a YAML file per environment (dev, staging, prod) with config values. On deploy, these are applied to DECREE. The repo is the source of truth for defaults/baselines.

3. **Runtime overrides via DECREE** — operators can change values at runtime via CLI/SDK without redeploying. These runtime changes override the baseline from git.

## The Flow

```
git repo                          DECREE
├── config/
│   ├── schema.yaml          →   Schema (auto-synced on deploy)
│   ├── values.dev.yaml      →   Config values (applied on deploy to dev tenant)
│   ├── values.staging.yaml  →   Config values (applied on deploy to staging tenant)
│   └── values.prod.yaml     →   Config values (applied on deploy to prod tenant)
```

### On deploy:
```bash
# Import schema + auto-publish if changed (checksum dedup skips if unchanged)
decree schema import --publish config/schema.yaml

# Apply baseline values (merge mode — default)
decree config import <tenant-id> config/values.prod.yaml --description "deploy $(git rev-parse --short HEAD)"
```

### At runtime:
- Operators use `decree config set` or the SDK to override specific values
- The override creates a new config version on top of the imported baseline
- Rollback to the baseline version is always possible

## Decisions

### 1. Schema sync: `--publish` flag
`decree schema import --publish` imports the schema YAML and auto-publishes if a new version was created. Single command, no new abstractions.

### 2. Config import modes
Three modes for different deployment strategies:

| Mode | Flag | Behavior | Default |
|------|------|----------|---------|
| **Merge** | `--mode merge` | Update fields from YAML that differ, keep runtime overrides for fields not in YAML | **Yes (default)** |
| **Replace** | `--mode replace` | Full replace — git always wins, all runtime overrides wiped | No |
| **Defaults** | `--mode defaults` | Only set fields that have no value yet, never overwrite | No |

**Merge semantics (default):**
- Field in YAML, different from current → update
- Field in YAML, same as current → skip
- Field in YAML, not in current config → set
- Field NOT in YAML → untouched (runtime overrides survive)

**Replace semantics:**
- All fields from YAML are set (creates new version with all values)
- Fields NOT in YAML are not carried forward
- Current behavior of `decree config import`

**Defaults semantics:**
- Field in YAML, no current value → set
- Field in YAML, already has a value → skip (regardless of YAML value)

**Why merge is the default:**
- Replace is destructive — accidentally wiping runtime overrides on every deploy is a bad default
- Defaults is too passive — can't push updated baselines from git
- Merge is the safest middle ground — git changes flow through, runtime overrides survive

### 3. Environment-specific tenants
One tenant per environment: `acme-dev`, `acme-staging`, `acme-prod`. Cleanest isolation.

### 4. Schema defaults vs config values
Complementary — schema `default` is used when a field has no value set at all. YAML values override schema defaults. Runtime overrides override YAML values.

Priority: `runtime override > YAML baseline > schema default`

## Implementation Needed

- [ ] `--publish` flag on `decree schema import` (and adminclient `ImportSchema`)
- [ ] `--mode` flag on `decree config import` (merge/replace/defaults)
- [ ] Merge mode in ConfigService `ImportConfig` RPC
- [ ] Defaults mode in ConfigService `ImportConfig` RPC
- [ ] Proto: add `ImportMode` enum to `ImportConfigRequest`
- [ ] Update SDK adminclient `ImportConfig` to accept mode
- [ ] Tests for all three modes
- [ ] Update docs

## Related
- CLI power tools effort (08 Phase 3) — `decree seed` command wraps both
- Config import/export — needs mode extension
- Schema import — needs `--publish` flag
