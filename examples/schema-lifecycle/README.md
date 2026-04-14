# Schema Lifecycle

The full administrative workflow: create, update, publish, and manage schemas.

## What this demonstrates

- Creating a schema with typed fields
- Publishing a version (makes it immutable and assignable to tenants)
- Updating a schema (adds fields, creates a new draft version)
- Creating a tenant on a specific schema version
- Listing schemas

## Run it

```bash
go run .
```

This example creates and cleans up its own schema — no seed data required
(but the server must be running).

## Expected output

```
Creating schema...
  Created: <uuid> (v1, draft)
Publishing v1...
  Published: v1
Updating schema (adding webhook.url)...
  Updated: v2 (draft)
Publishing v2...
  Published: v2
Creating tenant...
  Tenant: <uuid> (schema v2)
Total schemas: 2
Cleaning up...
Done.
```

## Learn more

- [adminclient package](https://pkg.go.dev/github.com/opendecree/decree/sdk/adminclient) — full admin API reference
- [Schema YAML format](../../docs/api/) — import/export schema definitions
- [Previous: Optimistic Concurrency](../optimistic-concurrency/) | [Next: Environment Bootstrap →](../environment-bootstrap/)
