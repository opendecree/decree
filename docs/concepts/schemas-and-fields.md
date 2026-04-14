# Schemas & Fields

A **schema** defines the structure of a configuration — what fields exist, their types, constraints, and defaults. Schemas enforce consistency: every config value must belong to a field defined in the schema.

## Schema Lifecycle

```
Create (draft v1) → Update (draft v2, v3...) → Publish (immutable) → Assign to tenant
```

1. **Create** — define fields, types, and constraints. Creates version 1 as a draft.
2. **Update** — add, modify, or remove fields. Each update creates a new draft version.
3. **Publish** — mark a version as immutable. Only published versions can be assigned to tenants.
4. **Assign** — create a tenant bound to a published schema version.

Published versions are immutable — you cannot change their fields. To evolve a schema, create a new version and publish it.

## Schema YAML Format

Schemas are defined in YAML for import/export. The format uses syntax version `v1`:

```yaml
# yaml-language-server: $schema=../../schemas/schema-yaml.json
syntax: "v1"
name: payments
description: Payment processing configuration
version_description: Add timeout field

fields:
  payments.enabled:
    type: bool
    description: Whether payment processing is active
    default: "true"

  payments.fee_rate:
    type: number
    description: Fee percentage per transaction
    nullable: true
    constraints:
      minimum: 0
      maximum: 1

  payments.currency:
    type: string
    description: Default settlement currency
    constraints:
      enum: [USD, EUR, GBP]

  payments.max_retries:
    type: integer
    description: Maximum retry attempts
    constraints:
      minimum: 0
      maximum: 10

  payments.timeout:
    type: duration
    description: Payment processing timeout

  payments.webhook:
    type: url
    description: Webhook endpoint for payment events
    nullable: true

  payments.metadata_schema:
    type: json
    description: Custom metadata shape
    constraints:
      json_schema: |
        {"type": "object", "properties": {"ref": {"type": "string"}}}

  payments.old_fee:
    type: string
    deprecated: true
    redirect_to: payments.fee_rate
```

A [JSON Schema](../../schemas/schema-yaml.json) is available for editor validation and autocomplete.

### Required fields

| Field | Required | Description |
|-------|----------|-------------|
| `syntax` | Yes | Must be `"v1"` |
| `name` | Yes | Unique slug: lowercase alphanumeric + hyphens, 1-63 chars |
| `fields` | Yes | At least one field definition |
| `description` | No | Schema description |
| `version` | No | Informational — server assigns the next version on import |
| `version_description` | No | What changed in this version |

## Field Types

Every field has a type that determines what values are accepted and how they're represented on the wire (via the [TypedValue](typed-values.md) oneof):

| YAML type | Proto type | Go type | Example value |
|-----------|-----------|---------|---------------|
| `integer` | `int64` | `int64` | `42`, `-1` |
| `number` | `double` | `float64` | `3.14`, `0.025` |
| `string` | `string` | `string` | `"hello"`, `"USD"` |
| `bool` | `bool` | `bool` | `true`, `false` |
| `time` | `google.protobuf.Timestamp` | `time.Time` | `2025-01-15T09:30:00Z` |
| `duration` | `google.protobuf.Duration` | `time.Duration` | `24h`, `30s`, `500ms` |
| `url` | `string` | `string` | `https://example.com/hook` |
| `json` | `string` | `string` | `{"key": "value"}` |

Type safety is enforced at the wire level — sending a string to an integer field is rejected by the server.

## Constraints

Constraints are optional validation rules checked on every write (including import). Use OAS-style naming:

| Constraint | Applies to | Description |
|-----------|-----------|-------------|
| `minimum` | integer, number, duration | Minimum allowed value, inclusive (`>=`). |
| `maximum` | integer, number, duration | Maximum allowed value, inclusive (`<=`). |
| `exclusiveMinimum` | integer, number, duration | Exclusive minimum, strict (`>`). |
| `exclusiveMaximum` | integer, number, duration | Exclusive maximum, strict (`<`). |
| `minLength` | string | Minimum allowed string length. |
| `maxLength` | string | Maximum allowed string length. |
| `pattern` | string | Regular expression (RE2 syntax) the value must match. |
| `enum` | any type | Allowed values. The value must be one of these. |
| `json_schema` | json | JSON Schema document for structural validation. |

URL fields are always validated for absolute URL format, even without explicit constraints.

Constraints are validated at schema creation time — applying a constraint to an incompatible type (e.g. `minimum` on a string, `minLength` on an integer) is rejected.

### Constraint examples

```yaml
# Integer range (inclusive)
constraints:
  minimum: 0
  maximum: 100

# Number range (exclusive — value must be > 0 and < 1)
constraints:
  exclusiveMinimum: 0
  exclusiveMaximum: 1

# String length + pattern
constraints:
  minLength: 3
  maxLength: 50
  pattern: '^[A-Z]+$' # uppercase only

# Enum
constraints:
  enum: [dev, staging, prod]

# JSON Schema
constraints:
  json_schema: |
    {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}
```

## Field Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `nullable` | bool | `false` | Whether the field accepts null values. See [Typed Values — Null](typed-values.md). |
| `deprecated` | bool | `false` | Mark the field as deprecated. Deprecated fields are still readable. |
| `redirect_to` | string | — | When deprecated, reads can be redirected to this field path. |
| `default` | string | — | Default value for the field (encoded as string). |
| `description` | string | — | Human-readable description of the field's purpose. |

## Import/Export Semantics

### Import

- Schema lookup by `name`:
    - **Doesn't exist** → creates new schema with v1
    - **Exists, fields differ** → creates the next version as a draft
    - **Exists, fields identical** → returns `AlreadyExists` (no-op)
- Imported versions are **drafts** by default. Use `--publish` (CLI) or `auto_publish` (API) to auto-publish on import.
- The `version` field in YAML is informational — server assigns the next version
- Full-replace semantics: the YAML defines the complete field set, not a diff

### Export

- Exports a specific version (or latest) as YAML
- Server-generated fields excluded: `id`, `checksum`, `published`, `created_at`

## Strict Mode

When writing config values, the server operates in **strict mode**: writes to field paths not defined in the tenant's schema are rejected. This prevents typos and undeclared fields from entering the config.

## Related

- [Typed Values](typed-values.md) — the TypedValue type system
- [Tenants](tenants.md) — how schemas are assigned to tenants
- [API Reference — SchemaService](../api/api-reference.md) — full RPC details
- [CLI — decree schema](../cli/decree_schema.md) — managing schemas from the command line
