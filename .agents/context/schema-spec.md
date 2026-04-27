# Schema Spec v0.1.0 — Design Context

Repo: `opendecree/decree`
Milestone: [Schema Spec v0.1.0](https://github.com/opendecree/decree/milestone/10)
Related: issue #117, discussion #116

## Goal

Publish a language-neutral description of the decree schema YAML format — a JSON Schema draft 2020-12 document — so third-party tooling (editors, CI linters, alt doc generators, code generators, format importers, LLM grounding, schema diff tools) can validate `decree.schema.yaml` files offline without shelling out to the Go reference parser or reimplementing its rules.

Version the meta-schema with SemVer, starting at `0.1.0` to signal it is pre-stable and subject to change. Promote to `1.0.0` once the shape is frozen.

## Non-goals

- **Replacing Go-side semantic validation.** `internal/schema/validate_constraints.go` remains authoritative for cross-field rules. The meta-schema covers layer-1 structure (valid keys, required fields, type-of-type-values, per-type constraint compatibility). Semantic rules that a JSON Schema cannot express (e.g. referential integrity between `redirect_to` and existing field paths) stay in Go.
- **Runtime validation of config values against schema.** That is `internal/validation/`, a separate concern.
- **Covering every conceivable extension point.** The `x-*` extension mechanism (below) is the escape hatch for things the core format doesn't model.

## Use cases

Primary use cases driving v0.1.0:

1. **Editor IntelliSense** — schemastore.org integration so VS Code / Zed / IntelliJ / Helix auto-apply the meta-schema to `*.decree.schema.yaml` files.
2. **CI validation** — `check-jsonschema` catches malformed schemas before they reach the server.
3. **LLM grounding** — feed the meta-schema to a model so it emits valid decree schemas for demos and onboarding.

Enabled once v0.1.0 ships (community-driven, not in scope):

- Alt doc generators in Python/TS/Rust
- Schema diff / migration tools
- `schema.yaml → typed code` generators (Python dataclasses, TS interfaces, Terraform modules)
- Format importers (OpenAPI → decree, JSON Schema → decree)
- Community schema registry / hub
- Property-based fuzz testing for internal decree tests
- UI schema editor driven by the meta-schema
- Spec governance — breaking vs additive changes become visible in meta-schema diffs

## Format changes in v0.1.0

All changes are **breaking**. Per project policy (no backward compat pre-production), no deprecation period.

| Change | Rationale |
|--------|-----------|
| Rename `syntax:` → `spec_version:` | Clearer intent; `$schema` handles the "which meta-schema" pointer |
| Add optional top-level `$schema:` | Points to meta-schema URL; survives serialization (unlike modeline comment) |
| Add optional top-level `$id:` | URN for federation (e.g. `urn:decree:schema:payments:v3`) |
| Enforce field-path regex `^[a-zA-Z_][a-zA-Z0-9_.-]*$` | Existing usage covered; prevents pathological keys |
| Reject unknown keys (`unevaluatedProperties: false`) | Typo prevention; matches OAS 3.1 |
| Reserve `x-*` prefix for vendor extensions | Extensibility without polluting core format; matches OAS 3.1 |
| `format:` stays free-form string | Recommend common values in docs, don't enforce (matches OAS) |
| Reserve top-level `validations:` (list of CEL rules) | Cross-field validation key — see [cel-validation.md](cel-validation.md). Parser accepts and stores in v0.1.0; engine ships separately. Reserving up-front avoids a v0.2.0 breaking change. |
| Reserve top-level `dependentRequired:` (JSON Schema 2020-12 keyword) | Declarative "field B required when field A present" — free of CEL. Native-substitutable cases should use it. |

### Final top-level shape (v0.1.0)

```yaml
# yaml-language-server: $schema=https://schemas.opendecree.io/schema/v0.1.0/decree.json

spec_version: v1                          # required, const "v1" (the decree format version)
$schema: https://schemas.opendecree.io/schema/v0.1.0/decree.json  # optional
$id: urn:decree:schema:payments:v3        # optional
name: payments                            # required, slug ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$
description: Payments service config      # optional
version: 3                                # optional int
version_description: Added refund limits  # optional
info:                                     # optional
  title: Payments
  author: Platform team
  contact: { name, email, url }
  labels: { team: platform }
fields:                                   # required, min 1 entry
  app.name:                               # keys match ^[a-zA-Z_][a-zA-Z0-9_.-]*$
    type: string                          # required, enum of 8 values
    description: ...
    default: ...
    nullable: false
    deprecated: false
    redirect_to: ...
    title: ...
    example: ...
    examples: { name: { value, summary } }
    externalDocs: { url (req), description }
    tags: [...]
    format: email                         # free-form string
    readOnly: false
    writeOnce: false
    sensitive: false
    constraints:                          # shape depends on type (see matrix)
      ...
    x-custom-extension: ...               # allowed via x-* prefix
  x-top-level-extension: ...              # allowed at any level

# Optional top-level keys for cross-field validation (reserved in v0.1.0):
validations:                              # CEL rule list — engine ships separately
  - path: payments
    rule: "self.payments.min < self.payments.max"
    message: "min must be < max"
dependentRequired:                        # JSON Schema 2020-12 keyword
  payments.refunds_enabled: [payments.refund_window]
```

See [cel-validation.md](cel-validation.md) for the full design of `validations:` and the prior-art survey driving the binding shape (`self`).

### Field type system

Unchanged in v0.1.0. 8 types:

`integer`, `number`, `string`, `bool`, `time`, `duration`, `url`, `json`

### Constraint matrix

| Constraint | numeric (integer, number, duration) | string | json | other (bool, time, url) |
|---|---|---|---|---|
| `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum` | ✓ | ✗ | ✗ | ✗ |
| `minLength`, `maxLength`, `pattern` | ✗ | ✓ | ✗ | ✗ |
| `json_schema` | ✗ | ✗ | ✓ | ✗ |
| `enum` | ✓ | ✓ | ✓ | ✓ |

Meta-schema encodes this via `allOf` with 4 `if/then` branches keyed on `type`.

## Meta-schema approach

- **Draft:** JSON Schema 2020-12 (`$schema: https://json-schema.org/draft/2020-12/schema`)
- **Per-type constraints:** `allOf` of `if/then` branches (better error messages than `oneOf` in ajv / python-jsonschema)
- **Unknown keys:** `unevaluatedProperties: false` at every object level (not `additionalProperties: false` — the latter breaks composition with `allOf`)
- **Extensions:** `patternProperties: { "^x-": {} }` at every object level
- **Composition:** `$defs` + `$ref` internal; no cross-file `$ref` (revised from the original "split + bundle" plan after the format proved small enough that splitting cost more than it saved).
- **Source of truth:** two single-file meta-schemas under `schemas/v0.1.0/` — `decree-schema.yaml` (validates `*.decree.schema.yaml`) and `decree-config.yaml` (validates `*.decree.config.yaml`). YAML is the human-edited source; JSON copies are generated by `scripts/yaml-to-json.py` and committed alongside for tooling consumers (schemastore.org, IDE language servers) that prefer JSON.

### Breaking vs additive (for future minor/patch bumps)

| Additive (minor bump) | Breaking (major bump) |
|-----------------------|-----------------------|
| New optional property | Removing a property |
| Loosening a constraint (higher `maxLength`, wider `pattern`) | Tightening a constraint |
| New `enum` value | Removing an `enum` value, changing a `type` |
| | Making optional required |

## File conventions

- **Canonical filenames:** `decree.schema.yaml`, `decree.config.yaml`
- **Generic globs:** `*.decree.schema.yaml`, `*.decree.config.yaml` (for repos with multiple schemas)
- **Modeline:** `# yaml-language-server: $schema=https://schemas.opendecree.io/schema/v0.1.0/decree.json` on line 1 of every example
- **CLI stays filename-agnostic** — `decree apply some-other-name.yaml` must keep working. The convention drives editor discovery only.

## Publishing

- **URL pattern:** `https://schemas.opendecree.io/schema/v{MAJOR}.{MINOR}.{PATCH}/decree.json`
- **Pre-stable:** full SemVer in path (`/v0.1.0/`)
- **Post-1.0.0:** switch to major-only paths (`/v1/`) with permanent redirects from historical full-SemVer URLs
- **Hosting:** TBD (GitHub Pages on a dedicated repo, or static-hosted redirect to raw GitHub content). Must be HTTPS, stable, `Content-Type: application/schema+json`.
- **schemastore.org:** PR against `SchemaStore/schemastore` adding entry to `src/api/json/catalog.json`:
  ```json
  {
    "name": "OpenDecree Schema",
    "description": "OpenDecree configuration schema",
    "fileMatch": ["decree.schema.yaml", "decree.schema.yml", "*.decree.schema.yaml", "*.decree.schema.yml"],
    "url": "https://schemas.opendecree.io/schema/v0.1.0/decree.json"
  }
  ```

## CI

- **Primary tool:** `check-jsonschema` (Python, wraps `jsonschema` library, excellent error messages, built-in YAML support, default Draft 2020-12)
- **Backup:** `ajv-cli` v5+ if Python is unavailable
- **Run on:** every PR, every push to main
- **Target files:** all checked-in `*.decree.schema.yaml` (and legacy names until migrated) under `examples/`, `e2e/`, `docs/`
- **Known-invalid fixtures:** a `testdata/invalid/` directory with YAMLs that must fail validation — proves the meta-schema actually rejects what it should

## Docs to produce

- `docs/concepts/schema-format.md` — canonical user-facing spec (every field, every type, every constraint, examples)
- `docs/concepts/meta-schema.md` — what the meta-schema covers, non-goals (semantic validation stays Go-side), SemVer + stability policy
- `docs/getting-started.md` — update examples to show `# yaml-language-server:` modeline and `decree.schema.yaml` naming
- `README.md` — link to published meta-schema URL

## Issue breakdown (phased)

### Phase A — Format changes (breaking, in-tree)

- #118 — Rename `syntax:` → `spec_version:` across parser, fixtures, examples, docs, e2e, CLI error messages _(merged)_
- #119 — Add optional top-level `$schema:` and `$id:` fields to parser (validated if present)
- #120 — Enforce field-path regex `^[a-zA-Z_][a-zA-Z0-9_.-]*$` in Go parser
- #121 — Reject unknown keys at parse time + reserve `x-*` extension prefix
- #122 — Rename fixture files to `*.decree.schema.yaml` / `*.decree.config.yaml`

### Phase B — Meta-schema

- #123 — Author meta-schemas under `schemas/v0.1.0/` (`decree-schema.yaml` + `decree-config.yaml`, single-file each, JSON copies generated)
- #124 — CI: validate every checked-in schema YAML + known-invalid fixtures via `make validate-meta-schemas`

### Phase C — Publishing

- #125 — Host meta-schema at `https://schemas.opendecree.io/schema/v0.1.0/decree.json`
- #126 — Submit schemastore.org PR

### Phase D — Docs

- #127 — Write `docs/concepts/schema-format.md`, `docs/concepts/meta-schema.md`; update `docs/getting-started.md` with modeline + new filenames; update `docs/concepts/schemas-and-fields.md` to cross-link the new concept docs; link meta-schema from `README.md`

### Cross-cutting

- #129 — Pluggable schema parser. `Parser` interface + package-level registry in `internal/schema/parser.go` (and symmetric in `internal/config/parser.go`). Each spec version is a single sibling file (`parser_v1.go`, future `parser_v2.go`) that declares a parser struct and registers it via `init()`. The service layer dispatches through `schema.Dispatch(yaml)` on import and `schema.MarshalSchemaAt(s, version)` on export; layer-2 semantic checks run on the proto value after dispatch so they're shared across versions. Adding v2 means landing one new file; nothing else in the codebase changes.
- #76 (Phase 1) — Reserve `validations:` and `dependentRequired:` keys in meta-schema + parser. No engine; rules round-trip through ImportSchema/GetSchema unevaluated. See [cel-validation.md](cel-validation.md). Must land in v0.1.0 to lock the schema shape.

## Open questions

- **Hosting target for `schemas.opendecree.io`** — dedicated GitHub Pages repo? Cloudflare redirect to raw GitHub content? Needs DNS + CORS setup.
- **Bundling tool** — hand-rolled Python script vs off-the-shelf (e.g. `json-dereference-cli`). Go with off-the-shelf if one exists and is maintained.
- **Does the CLI emit `$schema`/`$id` on export?** — `decree schema export` should probably inject `$schema` by default, make `$id` opt-in.
- **Post-v1.0.0 URL migration** — when the spec promotes to 1.0.0, keep `/v0.1.0/` live forever or redirect? Preserve forever matches OpenAPI's dated-URL practice.

## References

- Issue: https://github.com/opendecree/decree/issues/117
- Discussion: https://github.com/opendecree/decree/discussions/116
- JSON Schema 2020-12: https://json-schema.org/draft/2020-12/schema
- OpenAPI 3.1 meta-schema: https://github.com/OAI/OpenAPI-Specification/tree/main/schemas/v3.1
- AsyncAPI schemas: https://github.com/asyncapi/spec-json-schemas
- schemastore.org: https://github.com/SchemaStore/schemastore
- check-jsonschema: https://github.com/python-jsonschema/check-jsonschema
- yaml-language-server modeline: https://github.com/redhat-developer/yaml-language-server#using-inlined-schema
