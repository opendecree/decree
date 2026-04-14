# Schema YAML Enrichment

**Status:** Complete
**Started:** 2026-04-09

---

## Goal

Enrich the schema YAML format with OAS-inspired metadata so we can generate rich docs, power the GUI, and provide better DX. All additions are optional and backward-compatible — existing v1 schemas remain valid.

## Extended Schema YAML Format

### Schema-Level Additions

```yaml
syntax: "v1"
name: payments
description: "Payment processing configuration"

# NEW — schema-level metadata
info:
  title: "Payment Configuration"        # Human-friendly name (OAS: info.title)
  author: "payments-team"               # Schema owner
  contact:
    name: "Payments Team"               # OAS: info.contact.name
    email: "payments@example.com"       # OAS: info.contact.email
    url: "https://wiki.example.com/pay" # OAS: info.contact.url
  labels:                               # Key-value metadata for filtering
    team: payments
    domain: billing
    environment: production

fields:
  # ...
```

### Field-Level Additions

```yaml
fields:
  payments.fee_rate:
    type: number
    description: "Fee percentage charged per transaction"

    # NEW — display metadata
    title: "Fee Rate"                    # OAS: title — human-friendly name
    example: 0.025                       # OAS: example — single example value
    examples:                            # OAS: examples — named examples
      low_fee:
        value: 0.01
        summary: "Low-volume discount"
      standard:
        value: 0.025
        summary: "Standard rate"
    externalDocs:                        # OAS: externalDocs
      description: "Fee calculation guide"
      url: "https://wiki.example.com/fees"

    # NEW — categorization
    tags: [billing, critical]            # Grouping beyond dot-prefix

    # NEW — type hint within base type
    format: "percentage"                 # OAS: format — semantic hint

    # NEW — access control hints
    readOnly: false                      # OAS: readOnly — system-managed, not user-editable
    writeOnce: false                     # Set once, immutable after (like IDs)

    # NEW — security
    sensitive: false                     # Mask in logs/GUI (API keys, secrets)

    # EXISTING (unchanged)
    nullable: true
    deprecated: false
    redirect_to: ""
    default: ""
    constraints:
      minimum: 0
      maximum: 1
```

## Field Reference (all new properties)

### Schema-level: `info`

| Property | Type | OAS Equivalent | Description |
|----------|------|---------------|-------------|
| `info.title` | string | `info.title` | Human-friendly schema name |
| `info.author` | string | — | Schema owner identifier |
| `info.contact.name` | string | `info.contact.name` | Contact person/team |
| `info.contact.email` | string | `info.contact.email` | Contact email |
| `info.contact.url` | string | `info.contact.url` | Contact URL |
| `info.labels` | map[string]string | — | Key-value metadata for filtering |

### Field-level

| Property | Type | OAS Equivalent | Description |
|----------|------|---------------|-------------|
| `title` | string | `title` | Human-friendly field name |
| `example` | any | `example` | Single example value |
| `examples` | map | `examples` | Named examples with value + summary |
| `externalDocs.description` | string | `externalDocs.description` | Link description |
| `externalDocs.url` | string | `externalDocs.url` | External doc URL |
| `tags` | []string | x-tags | Grouping/categorization tags |
| `format` | string | `format` | Semantic type hint (email, semver, percentage, etc.) |
| `readOnly` | bool | `readOnly` | System-managed, not user-editable |
| `writeOnce` | bool | — | Immutable after first set |
| `sensitive` | bool | — | Mask in logs/GUI |

## Common Format Values

Informational hints — not enforced by validation (like OAS format):
- `email`, `uri`, `hostname`, `ipv4`, `ipv6` — string subtypes
- `date`, `date-time` — time subtypes (already enforced by `time` type)
- `semver`, `iso-country`, `iso-currency`, `uuid` — domain formats
- `percentage`, `basis-points` — number subtypes
- `cron` — duration/string subtype

## Implementation Plan

### Proto Changes
- [ ] Add `info` field to `Schema` message (new `SchemaInfo` message)
- [ ] Add `title`, `example`, `examples`, `external_docs`, `tags`, `format`, `read_only`, `write_once`, `sensitive` to `SchemaField` message

### YAML Changes
- [ ] Extend `SchemaYAML` struct with `Info` field
- [ ] Extend `SchemaFieldYAML` struct with all new properties
- [ ] Update marshal/unmarshal functions
- [ ] Update `validateSchemaYAML` — new fields are optional, no new required fields

### JSON Schema Update
- [ ] Update `schemas/schema-yaml.json` with new optional properties

### Downstream Updates
- [ ] `sdk/tools/docgen` — use title, example, tags, externalDocs in generated markdown
- [ ] `sdk/tools/validate` — update YAML types (new fields are passthrough, no validation rules)
- [ ] `sdk/tools/seed` — update YAML types
- [ ] `sdk/adminclient` — update `Schema`/`Field` types with new fields
- [ ] `sdk/configclient` — no changes needed (reads values, not schema metadata)
- [ ] `sdk/configwatcher` — no changes needed

### Behavioral Impact
- `readOnly` — server enforces: reject writes from non-system actors
- `writeOnce` — server enforces: reject writes if field already has a value
- `sensitive` — server masks value in audit logs; CLI/GUI mask display
- All other new fields are informational (no enforcement)

## Key Decisions
- **Syntax stays v1** — all additions are optional, backward-compatible
- **OAS naming** — camelCase for consistency with OAS (title, readOnly, externalDocs)
- **format is a hint** — not validated (like OAS format), but tooling can use it
- **examples use OAS structure** — named map with value + summary
- **info block mirrors OAS info** — familiar to API developers
