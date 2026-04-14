# CLI Tool

**Status:** Phase 1+2 Complete
**Started:** 2026-03-26

---

## Goal

Single CLI binary (`ccs`) for managing the Central Config Service. Covers all admin and runtime operations plus power tools for documentation generation, diffing, and bulk operations. Built on the existing SDKs (configclient, adminclient, configwatcher).

## Architecture

```
cmd/ccs/                    # CLI entry point
├── main.go                 # cobra root command
├── schema.go               # ccs schema {create,get,list,update,publish,delete,export,import}
├── tenant.go               # ccs tenant {create,get,list,update,delete}
├── config.go               # ccs config {get,set,get-all,export,import,versions,rollback}
├── watch.go                # ccs watch — live stream of config changes
├── locks.go                # ccs lock {set,remove,list}
├── audit.go                # ccs audit {query,usage,unused}
├── docs.go                 # ccs docs {generate}
├── diff.go                 # ccs diff — compare two config versions
├── validate.go             # ccs validate — validate YAML against schema
├── seed.go                 # ccs seed — create schema + tenant + config from YAML
├── dump.go                 # ccs dump — export everything for backup
└── output.go               # shared output formatting (table, json, yaml)
```

Single binary, subcommand groups. Uses cobra for CLI framework.

## Command Groups

### Core Operations

| Command | SDK | Description |
|---------|-----|-------------|
| `ccs schema create` | adminclient | Create schema from flags or YAML file |
| `ccs schema get` | adminclient | Show schema (latest or specific version) |
| `ccs schema list` | adminclient | List all schemas |
| `ccs schema update` | adminclient | Add/remove fields, create new version |
| `ccs schema publish` | adminclient | Publish a draft version |
| `ccs schema delete` | adminclient | Delete schema (cascades) |
| `ccs schema export` | adminclient | Export schema to YAML |
| `ccs schema import` | adminclient | Import schema from YAML file |
| `ccs tenant create` | adminclient | Create tenant on a published schema |
| `ccs tenant get` | adminclient | Show tenant details |
| `ccs tenant list` | adminclient | List tenants (optionally by schema) |
| `ccs tenant update` | adminclient | Update name or schema version |
| `ccs tenant delete` | adminclient | Delete tenant (cascades) |
| `ccs config get` | configclient | Get a single field value |
| `ccs config get-all` | configclient | Get all config values |
| `ccs config set` | configclient | Set a single field value |
| `ccs config set-many` | configclient | Set multiple values (from flags or YAML) |
| `ccs config export` | adminclient | Export config to YAML |
| `ccs config import` | adminclient | Import config from YAML file |
| `ccs config versions` | adminclient | List config versions |
| `ccs config rollback` | adminclient | Rollback to a previous version |
| `ccs watch` | configwatcher | Stream live config changes (like `tail -f`) |
| `ccs lock set` | adminclient | Lock a field |
| `ccs lock remove` | adminclient | Unlock a field |
| `ccs lock list` | adminclient | List field locks |
| `ccs audit query` | adminclient | Query the audit log |
| `ccs audit usage` | adminclient | Show field usage stats |
| `ccs audit unused` | adminclient | Find unused fields |

### Power Tools

| Command | Description |
|---------|-------------|
| `ccs docs generate` | Generate markdown documentation from a schema (exported YAML → README with field descriptions, types, constraints, defaults) |
| `ccs diff` | Diff two config versions for a tenant (shows added/removed/changed fields) |
| `ccs validate` | Validate a schema or config YAML file locally without importing |
| `ccs seed` | Bootstrap: create schema from YAML, publish, create tenant, optionally apply config — all in one command |
| `ccs dump` | Full backup: export schema YAML + config YAML + locks for a tenant |

## Global Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--server` | `CCS_SERVER` | `localhost:9090` | gRPC server address |
| `--subject` | `CCS_SUBJECT` | — | x-subject header (actor identity) |
| `--role` | `CCS_ROLE` | `superadmin` | x-role header |
| `--tenant-id` | `CCS_TENANT_ID` | — | x-tenant-id header (for auth) |
| `--token` | `CCS_TOKEN` | — | JWT bearer token (alternative to metadata) |
| `--output` / `-o` | — | `table` | Output format: table, json, yaml |
| `--insecure` | `CCS_INSECURE` | `true` | Skip TLS (for local dev) |

## Output Formatting

All commands support `--output table|json|yaml`:
- **table** — human-readable aligned columns (default for terminal)
- **json** — machine-readable, one JSON object per result
- **yaml** — human-readable structured output

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- SDKs: configclient, adminclient, configwatcher
- `gopkg.in/yaml.v3` — YAML parsing for docs/validate/seed

## Implementation Plan

### Phase 1: Foundation + Core CRUD (completed)
- [x] Cobra root command with global flags, connection setup
- [x] Output formatting (table/json/yaml)
- [x] `ccs schema` subcommands (create/get/list/publish/delete/export/import)
- [x] `ccs tenant` subcommands (create/get/list/delete)
- [x] `ccs config` subcommands (get/set/get-all/set-many)

### Phase 2: Versioning + Streaming (completed)
- [x] `ccs config versions` / `ccs config rollback`
- [x] `ccs config export` / `ccs config import`
- [x] `ccs watch` — live change stream
- [x] `ccs lock` subcommands (set/remove/list)
- [x] `ccs audit` subcommands (query/usage/unused)
- [x] Unit tests — 17 tests: command structure, arg validation, output formatting, parseDuration

### Phase 3: Power Tools
- [ ] `ccs docs generate` — schema → markdown docs
- [ ] `ccs diff` — config version diffing
- [ ] `ccs validate` — offline YAML validation
- [ ] `ccs seed` — bootstrap from YAML
- [ ] `ccs dump` — full tenant backup

### Phase 4: Polish
- [ ] Shell completion (bash, zsh, fish via cobra)
- [ ] Man page generation
- [ ] Homebrew formula / goreleaser config
