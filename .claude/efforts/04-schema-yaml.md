# Schema YAML Import/Export

**Status:** Complete
**Started:** 2026-03-25

---

## Goal

Enable schema portability via a human-readable YAML format. Used for:
- Exporting schemas for backup, review, or version control
- Importing schemas into new environments or across instances
- Human authoring of schemas outside the API

## YAML Format (syntax v1)

```yaml
syntax: "v1"
name: payments
description: Payment processing configuration
version: 3
version_description: Add max_retries field

fields:
  payments.settlement.window:
    type: duration
    description: Settlement processing window
    default: "24h"
    constraints:
      minimum: 1
      maximum: 720

  payments.settlement.currency:
    type: string
    description: Default settlement currency
    nullable: true
    constraints:
      enum: [USD, EUR, GBP, ILS]

  payments.fee:
    type: string
    description: Fee percentage per transaction
    constraints:
      pattern: '^\d+(\.\d+)?%$'

  payments.max_retries:
    type: int
    description: Maximum retry attempts
    default: "3"
    constraints:
      minimum: 0
      maximum: 10

  payments.webhook_url:
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
    redirect_to: payments.fee
```

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Constraint naming | OAS-style: `minimum`, `maximum`, `pattern` | Industry standard, familiar to most engineers |
| Field keys | Dot-path as flat map key | Matches API usage, avoids nested ambiguity |
| Type names | Short: `int`, `string`, `duration`, `time`, `url`, `json` | Human-readable YAML |
| Syntax version | `syntax: "v1"` (protobuf-inspired) | Simple, separate from schema version |
| Published flag | Omitted from YAML | Import always creates a draft; publishing is an explicit API action |

## Type Mapping

| YAML type | Proto FieldType | Notes |
|-----------|----------------|-------|
| `int` | FIELD_TYPE_INT | |
| `string` | FIELD_TYPE_STRING | |
| `time` | FIELD_TYPE_TIME | |
| `duration` | FIELD_TYPE_DURATION | |
| `url` | FIELD_TYPE_URL | |
| `json` | FIELD_TYPE_JSON | |

## Constraint Mapping (YAML ↔ Proto)

| YAML (OAS-style) | Proto FieldConstraints | Applies to |
|-------------------|----------------------|------------|
| `minimum` | `min` | int, duration |
| `maximum` | `max` | int, duration |
| `pattern` | `regex` | string |
| `enum` | `enum_values` | any |
| `json_schema` | `json_schema` | json |

## Semantics

### Export
- Exports a specific schema version (or latest) as YAML
- Server-generated fields excluded: `id`, `checksum`, `published`, `created_at`, `parent_version`
- `version` is included for reference but ignored on import

### Import
- **Full replace** — YAML defines the complete field set, not a diff
- Schema lookup by `name`:
  - **Exists**: compute checksum of imported fields and compare to latest version's checksum. If identical → return existing schema (no-op, `AlreadyExists`). If different → create next version with imported fields.
  - **Doesn't exist**: create new schema with v1
- `version` in YAML is informational — server assigns the next version number
- Always creates as draft (unpublished)
- `syntax` field is required — reject unknown syntax versions
- `name` and `fields` are required; `description` and `version_description` are optional
- Checksum is computed server-side

## Implementation Plan

### 1. YAML struct definitions (`internal/schema/yaml.go`)
- Go structs with `yaml` tags matching the format above
- Conversion functions: proto → YAML struct, YAML struct → proto
- Constraint name translation (OAS ↔ proto)
- Validation: required fields, known syntax version, known types

### 2. Export implementation (`internal/schema/service.go`)
- `ExportSchema` RPC: load schema version from store → convert to YAML struct → marshal
- Return as `bytes` in the proto response

### 3. Import implementation (`internal/schema/service.go`)
- `ImportSchema` RPC: unmarshal YAML → validate → convert to proto → delegate to CreateSchema/UpdateSchema logic
- If schema name exists: create new version with the imported fields
- If schema name doesn't exist: create new schema

### 4. Tests
- Unit tests for YAML ↔ proto roundtrip conversion
- Unit tests for constraint name mapping
- Unit tests for validation (missing syntax, unknown type, etc.)
- E2E test: export → modify → import → verify

### Files to create/modify
- `internal/schema/yaml.go` (new) — YAML types + conversion + validation
- `internal/schema/yaml_test.go` (new) — conversion and validation tests
- `internal/schema/service.go` — implement ExportSchema and ImportSchema RPCs
- `e2e/e2e_test.go` — add export/import e2e test
- `go.mod` — add `gopkg.in/yaml.v3` dependency
