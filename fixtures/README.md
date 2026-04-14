# Test Fixtures

Pre-built scenarios for development, testing, and demos. Each file is a self-contained `decree seed` YAML that creates a schema, tenant, and optionally sets config values and field locks.

## Usage

```bash
# Load a single fixture
decree seed fixtures/billing.yaml

# Load multiple fixtures for a rich dev environment
decree seed fixtures/billing.yaml
decree seed fixtures/showcase.yaml
decree seed fixtures/empty.yaml
```

## Fixtures

| File | Scenario | Fields | Config | Locks |
|------|----------|--------|--------|-------|
| **minimal.yaml** | Simplest possible setup | 2 (string, bool) | Yes | No |
| **billing.yaml** | Realistic payments config | 5 (int, number, bool, string+enum, duration) | Yes | 1 locked |
| **showcase.yaml** | All field types + metadata | 11 (all 8 types, deprecated, tags, all flags) | Yes | No |
| **empty.yaml** | Blank slate — no values set | 4 (string, int, bool, nullable string) | No | No |
| **draft.yaml** | Unpublished schema | 3 (string, bool, number) | No | No |

## What each fixture tests

- **minimal** — smoke test, getting started, simplest happy path
- **billing** — constraints (min/max, enum), write-once field, field locks, schema info block
- **showcase** — every field type, every metadata field (title, description, examples, format, externalDocs, tags, readOnly, writeOnce, sensitive, deprecated + redirectTo)
- **empty** — default values, null handling, blank config editor state
- **draft** — unpublished schema workflow, publish button, no tenants possible
