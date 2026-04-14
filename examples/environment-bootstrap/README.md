# Environment Bootstrap

Bootstrap a complete environment from a single YAML file using the seed tool.

## What this demonstrates

- Defining schema + tenant + config + locks in one YAML file
- Idempotent seeding — safe to run multiple times
- Field locking to prevent accidental changes to critical values
- The `seed.Run` API for programmatic environment setup

## Run it

```bash
go run .
```

This example creates and cleans up its own schema — no seed data required
(but the server must be running).

## Seed file format

See [env.yaml](env.yaml) for the full format. Key sections:

- `schema` — field definitions with types and constraints
- `tenant` — tenant name
- `config` — initial values
- `locks` — fields that cannot be modified

## Expected output

```
Seeding environment...
  Schema:  <uuid> (v1, created=true)
  Tenant:  <uuid> (created=true)
  Config:  v1 (imported=true)
  Locks:   1 applied
```

## Learn more

- [seed package](https://pkg.go.dev/github.com/opendecree/decree/sdk/tools/seed) — seed API reference
- [decree seed CLI](../../docs/cli/) — CLI equivalent
- [Previous: Schema Lifecycle](../schema-lifecycle/) | [Next: Config Validation →](../config-validation/)
