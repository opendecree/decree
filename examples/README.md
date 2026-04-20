# OpenDecree SDK Examples

Runnable examples demonstrating the OpenDecree Go SDK.
Each example is a standalone Go module you can copy into your own project.

## Setup

Start the decree server and seed example data:

```bash
# From this directory
make setup
```

This starts PostgreSQL, Redis, and the decree server via Docker Compose,
then creates an example schema, tenant, and initial config values.

The tenant ID is written to `.tenant-id` — examples read it automatically.

## Examples

| Example | What it shows | Server required |
|---------|--------------|-----------------|
| [quickstart](quickstart/) | Connect + read typed config values | Yes |
| [feature-flags](feature-flags/) | Live feature toggles with configwatcher | Yes |
| [live-config](live-config/) | HTTP server with hot-reloadable config | Yes |
| [multi-tenant](multi-tenant/) | Same schema, different tenant values | Yes |
| [optimistic-concurrency](optimistic-concurrency/) | Safe concurrent updates with CAS | Yes |
| [schema-lifecycle](schema-lifecycle/) | Create, publish, and manage schemas | Yes |
| [environment-bootstrap](environment-bootstrap/) | Bootstrap from a single YAML file | Yes |
| [config-validation](config-validation/) | Offline config validation (no server) | No |

## Running an example

```bash
# After make setup:
cd quickstart
go run .
```

Or run all examples as tests:

```bash
make test
```

## Seed files in this directory

| File | Role |
|------|------|
| `seed.yaml` | **Canonical full example.** Rich schema with all field types, metadata, and constraints. Used by `make setup` to bootstrap the SDK examples. |
| `demo.seed.yaml` | **Minimal** — 3 fields in ~15 lines. Drives the `assets/demo.gif` terminal recording and doubles as a copy-paste quickstart snippet. |

Both are standard `decree seed` inputs (schema + tenant + config in one doc) — either works as a starting point.

## Teardown

```bash
make down
```

## Using in your own project

Each example directory is a self-contained Go module. To use one as a starting point:

1. Copy the directory to your project
2. Remove the `replace` directives from `go.mod`
3. Run `go mod tidy`

## Learn more

- [Go SDK reference](https://pkg.go.dev/github.com/opendecree/decree/sdk)
- [API documentation](../docs/api/)
- [CLI reference](../docs/cli/)
