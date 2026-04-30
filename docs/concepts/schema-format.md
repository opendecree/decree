# Schema Format Reference

The canonical reference for the **decree.schema.yaml** format — every top-level key, every field-level option, every constraint, every cross-field rule. For the higher-level concepts (lifecycle, drafts vs published versions, strict mode), see [Schemas & Fields](schemas-and-fields.md). For the JSON Schema 2020-12 meta-schema that validates this format mechanically, see [Meta-schema](meta-schema.md).

> **Alpha Software** — OpenDecree is under active development. APIs, proto definitions, and the schema format may change without notice between versions. Not recommended for production use yet.

## File conventions

| Filename | Purpose |
|----------|---------|
| `decree.schema.yaml` | Canonical name when a repo holds one schema. |
| `*.decree.schema.yaml` | Generic glob when a repo holds multiple (e.g. `payments.decree.schema.yaml`). |

Place the modeline on line 1 of every file so editors with [yaml-language-server](https://github.com/redhat-developer/yaml-language-server) auto-apply schema validation and IntelliSense:

```yaml
# yaml-language-server: $schema=https://schemas.opendecree.dev/schema/v0.1.0/decree-schema.json
```

The CLI is filename-agnostic — `decree apply some-other-name.yaml` keeps working. The convention drives editor discovery only.

## Top-level shape

```yaml
# yaml-language-server: $schema=https://schemas.opendecree.dev/schema/v0.1.0/decree-schema.json

spec_version: v1                                # required, const "v1"
$schema: https://schemas.opendecree.dev/schema/v0.1.0/decree-schema.json  # optional
$id: urn:decree:schema:payments:v3              # optional URN

name: payments                                  # required slug
description: Payment service configuration      # optional
version: 3                                      # optional informational integer
version_description: Added refund_window field  # optional

info:                                           # optional
  title: Payments
  author: Platform team
  contact: { name: Pat, email: pat@example.com, url: https://wiki/team }
  labels: { team: platform }

fields:                                         # required, ≥1 entry
  payments.fee:
    type: number
    constraints: { minimum: 0, maximum: 1 }

dependentRequired:                              # optional
  payments.refunds_enabled: [payments.refund_window]

validations:                                    # optional, reserved for Phase 2
  - path: payments
    rule: "self.payments.min < self.payments.max"
    message: "min must be less than max"

x-vendor-key: "any extension data"              # x-* allowed at every level
```

### Required vs optional

| Key | Required | Notes |
|-----|----------|-------|
| `spec_version` | required | Must be the literal string `"v1"`. |
| `name` | required | Slug — `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, 1–63 chars. |
| `fields` | required | ≥1 entry. |
| `$schema` | optional | HTTPS URL pointing at the meta-schema. |
| `$id` | optional | URN of the form `urn:decree:schema:<segment>(:<segment>)*`. |
| `description` | optional | Free-form text. |
| `version` | optional | Informational integer ≥ 1. The server assigns the actual version. |
| `version_description` | optional | Description of what changed in this version. |
| `info` | optional | Ownership metadata — see [Info Object](#info-object). |
| `dependentRequired` | optional | Cross-field "B required when A present" rules — see [Cross-field rules](#cross-field-rules). |
| `validations` | optional | Reserved for CEL expression rules; engine ships in Phase 2 (issue [#76](https://github.com/opendecree/decree/issues/76)). |

Unknown top-level keys are rejected at import unless they begin with the `x-` prefix.

## Fields

`fields:` is a map keyed on **field path** with one entry per field.

A field path matches the regex `^[a-zA-Z_][a-zA-Z0-9_.-]*$`. Dots within the path act as conventional grouping prefixes (`payments.fee`, `payments.refunds.window`) — the convention drives UI grouping and tag-vs-path orthogonality, but the parser treats the path as one opaque key.

Two field paths cannot collide such that one is a strict prefix of the other (e.g. `payments` and `payments.fee` cannot coexist) — see issue [#194](https://github.com/opendecree/decree/issues/194).

### Field types

Every field declares one of eight types. The type drives wire encoding, runtime validation, and which constraints apply.

| YAML type | Proto type | Go type | Example value |
|-----------|------------|---------|---------------|
| `integer` | `int64` | `int64` | `42`, `-1` |
| `number` | `double` | `float64` | `3.14`, `0.025` |
| `string` | `string` | `string` | `"hello"`, `"USD"` |
| `bool` | `bool` | `bool` | `true`, `false` |
| `time` | `google.protobuf.Timestamp` | `time.Time` | `2025-01-15T09:30:00Z` |
| `duration` | `google.protobuf.Duration` | `time.Duration` | `24h`, `30s`, `500ms` |
| `url` | `string` | `string` | `https://example.com/hook` |
| `json` | `string` (JSON-encoded) | `string` | `{"key": "value"}` |

`url` fields are always validated for absolute-URL form, even without explicit constraints. `json` fields are always parsed as valid JSON.

Type safety is enforced at the wire level — sending a string to an integer field is rejected by the server.

### Field options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `type` | enum | required | One of the 8 types above. |
| `description` | string | — | Human-readable description of the field's purpose. |
| `default` | string | — | Default value, encoded as a string per the field's type. |
| `nullable` | bool | `false` | Whether the field accepts null values. See [Typed Values — Null](typed-values.md). |
| `deprecated` | bool | `false` | Mark the field as deprecated. Reads still work. |
| `redirect_to` | string | — | When deprecated, reads can transparently follow this path. Existence checked at import. |
| `title` | string | — | Display name (e.g. `"Fee Rate"` for path `payments.fee_rate`). |
| `example` | string | — | Single example value, encoded as a string per the field's type. |
| `examples` | map | — | Named examples — see [Named examples](#named-examples). |
| `format` | string | — | Semantic format hint (advisory, not enforced) — see [Format hints](#format-hints). |
| `tags` | list | — | Cross-cutting categories — see [Tags](#tags). |
| `readOnly` | bool | `false` | System-managed; not user-editable. |
| `writeOnce` | bool | `false` | Settable once; immutable after first write. |
| `sensitive` | bool | `false` | Mask in logs and UI. |
| `externalDocs` | object | — | Link to external documentation: `{ url: <required>, description: <optional> }`. |
| `constraints` | object | — | Validation rules — see [Constraints](#constraints). |

`x-*` extension keys are accepted at every level.

### Format hints

`format:` is a free-form string — the parser does not enforce any format value. Authors signal semantic intent for tooling and humans; tooling chooses whether to use it.

Recommended values (advisory only):

| `format:` | Meaning |
|-----------|---------|
| `email` | RFC 5322 email address. |
| `uri` / `url` | RFC 3986 URI — note `url` as a TYPE is enforced; format hint differentiates "URL string" within a `string` type. |
| `uuid` | RFC 4122 UUID. |
| `ipv4` / `ipv6` / `ipv6-zone` | IP address forms. |
| `hostname` / `idn-hostname` | Hostname / internationalized hostname. |
| `date` / `date-time` / `time` | RFC 3339 forms (use `time` field type for full datetime). |
| `regex` | A field whose value is itself a regular expression. |
| `semver` | [SemVer 2.0.0](https://semver.org). |
| `percentage` | A percentage value (string or number). |
| `color` | CSS color string (`#rrggbb`, `rgb(...)`, etc.). |
| `currency` | ISO 4217 three-letter code. |

Vendor-specific formats should use the `x-` prefix (e.g. `x-stripe-customer-id`).

## Constraints

Constraints attach to a field via the `constraints:` map. Available constraints depend on the field's type — applying an incompatible constraint is rejected at import.

| Constraint | numeric (integer, number, duration) | string | json | other (bool, time, url) |
|------------|:-:|:-:|:-:|:-:|
| `minimum` / `maximum` | ✓ | ✗ | ✗ | ✗ |
| `exclusiveMinimum` / `exclusiveMaximum` | ✓ | ✗ | ✗ | ✗ |
| `minLength` / `maxLength` | ✗ | ✓ | ✗ | ✗ |
| `pattern` | ✗ | ✓ | ✗ | ✗ |
| `json_schema` | ✗ | ✗ | ✓ | ✗ |
| `enum` | ✓ | ✓ | ✓ | ✓ |

Range sanity is checked at import — `minimum > maximum`, `minLength > maxLength`, and `exclusiveMinimum >= exclusiveMaximum` are all rejected.

### Constraint examples

```yaml
# Integer range, inclusive.
constraints:
  minimum: 0
  maximum: 100

# Number range, exclusive — value must be > 0 and < 1.
constraints:
  exclusiveMinimum: 0
  exclusiveMaximum: 1

# String length + pattern.
constraints:
  minLength: 3
  maxLength: 50
  pattern: '^[A-Z]+$'   # uppercase only (RE2)

# Enum on any type.
constraints:
  enum: [dev, staging, prod]

# Embedded JSON Schema for json fields.
constraints:
  json_schema: |
    {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}
```

The `pattern` constraint uses [RE2](https://github.com/google/re2/wiki/Syntax) syntax — a subset of PCRE without backtracking. ReDoS-safe by construction. An invalid pattern is rejected at import time with `InvalidArgument`; the schema is not stored.

## Cross-field rules

Two complementary mechanisms cover cross-field invariants. Pick the simpler one whenever it suffices.

### dependentRequired

Declarative "if field A is set, field B must also be set". Free, no expression engine. Matches the [JSON Schema 2020-12 keyword](https://json-schema.org/understanding-json-schema/reference/conditionals#dependentrequired) of the same name.

```yaml
fields:
  payments.refunds_enabled: { type: bool }
  payments.refund_window:   { type: duration, nullable: true }

dependentRequired:
  payments.refunds_enabled: [payments.refund_window]
```

Semantics:

- **Triggers on non-null.** A rule fires only when the trigger has a non-null value in the post-merge configuration. Setting the trigger to null clears the requirement.
- **Lint at import.** `ImportSchema` rejects rules where the trigger or any dependent does not name a defined field, where a trigger lists itself as a dependent, or where a dependent appears twice under the same trigger.
- **Runtime enforcement.** Every config write (`SetField`, `SetFields`, `ImportConfig`, `RollbackToVersion`) evaluates all rules against the post-merge state inside the same transaction. A rule violation rejects the write with `InvalidArgument`.

### validations (CEL)

Reserved for cross-field rules expressed in [Common Expression Language](https://github.com/google/cel-spec). The schema-format key is reserved in v0.1.0; the runtime engine ships in Phase 2 (issue [#76](https://github.com/opendecree/decree/issues/76)). v0.1.0 schemas with `validations:` parse and persist; rules become no-ops at write time until the engine ships.

```yaml
validations:
  - path: payments
    rule: "self.payments.min_amount < self.payments.max_amount"
    message: "min_amount must be less than max_amount"
    severity: error                # optional, default error
    reason: MIN_GE_MAX             # optional machine code
```

Reach for `validations:` when `dependentRequired:` cannot express the rule — typically arithmetic comparisons (`min < max`), conditional requirement based on a value (`if env == "prod" then audit_url required`), or multi-field invariants. See [`.agents/context/cel-validation.md`](https://github.com/opendecree/decree/blob/main/.agents/context/cel-validation.md) for the full design.

## Named examples

`example:` is a single value; `examples:` is a named map for showing multiple realistic shapes.

```yaml
fields:
  rate_limits:
    type: json
    description: Per-tenant rate limits.
    examples:
      basic:
        value: '{"requests_per_second": 100}'
        summary: Default rate limit for free-tier tenants.
      strict:
        value: '{"requests_per_second": 10, "burst": 5}'
        summary: Strict limit for trial accounts.
```

## Tags

Tags provide cross-cutting categorization independent of the path hierarchy. Useful for UI grouping, filtering, and documentation.

```yaml
fields:
  api.webhook_url:
    type: url
    tags: [integrations]

  payments.fee_rate:
    type: number
    tags: [billing, compliance]    # multi-tag

  payments.currency:
    type: string
    tags: [compliance]
```

Path prefixes (`api`, `payments`) and tags are independent dimensions. Real schemas often tag fields across multiple path groups to support views like "all compliance-related fields" or "all integrations".

## Info object

```yaml
info:
  title: Payments
  author: Platform team
  contact:
    name: Pat
    email: pat@example.com
    url: https://wiki/teams/platform
  labels:
    team: platform
    cost-center: "1234"
```

| Key | Type | Notes |
|-----|------|-------|
| `title` | string | Display title (free-form, human-friendly). |
| `author` | string | Owner name (person, team, or service). |
| `contact.name` | string | Contact name. |
| `contact.email` | string | Contact email (RFC 5322). |
| `contact.url` | string | Contact URL (e.g. team wiki). |
| `labels` | map<string,string> | Free-form key/value labels for filtering and categorization. |

## Vendor extensions (`x-*`)

Any object level — top-level, field, constraints, info, externalDocs, examples, validations entries — accepts keys with the `x-` prefix. The parser preserves them on round-trip but does not interpret them.

```yaml
x-internal-team: payments
fields:
  payments.fee:
    type: number
    x-monitoring-alert-threshold: 0.05
```

Use `x-*` for vendor- or team-specific metadata that doesn't belong in the core format.

## Import / export semantics

### Import

- **Schema lookup by `name`:**
  - Schema doesn't exist → creates a new schema with v1.
  - Schema exists, fields differ → creates the next version as a draft.
  - Schema exists, fields identical → returns `AlreadyExists` (no-op).
- Imported versions are **drafts** by default. Use `--publish` (CLI) or `auto_publish` (API) to auto-publish on import.
- The `version` field in YAML is informational — the server assigns the actual next version.
- **Full-replace** semantics: the YAML defines the complete field set, not a diff against the previous version.

### Export

- Exports a specific version (or latest) as YAML.
- Server-generated fields excluded from export: `id`, `checksum`, `published`, `created_at`.
- The exported document includes the `# yaml-language-server:` modeline and `$schema` pointer for round-trip editor support.

## Strict mode

When writing config values against a schema, the server operates in **strict mode**: writes to field paths not defined in the tenant's schema are rejected. This prevents typos and undeclared fields from entering the config. There is no "permissive" mode in v0.1.0.

## Related

- [Schemas & Fields](schemas-and-fields.md) — higher-level concepts: lifecycle, drafts vs published versions, evolution.
- [Meta-schema](meta-schema.md) — the JSON Schema 2020-12 meta-schema that mechanically validates files in this format.
- [Typed Values](typed-values.md) — how `type:` maps to the wire `TypedValue` oneof.
- [Tenants](tenants.md) — how schemas are bound to tenants for config writes.
- [API Reference — SchemaService](../api/api-reference.md) — the gRPC RPC surface.
- [CLI — `decree schema`](../cli/decree_schema.md) — managing schemas from the command line.
