---
title: decree seed
---

## decree seed

Seed a schema, tenant, and/or config from a YAML file

### Synopsis

Seed applies a YAML file against the server. The file may contain any combination of schema, tenant, config, and locks sections; the operation dispatches based on which are present:

  schema only                  → imports the schema
  tenant only                  → creates (or reuses) the tenant
  schema + tenant              → imports schema + creates tenant
  tenant + config (+ locks)    → reuses schema, creates tenant, imports config
  schema + tenant + config     → full combined envelope (legacy form)

In config-only mode, tenant.schema names an already-imported schema. If tenant.schema_version is omitted, the latest published version is used.

The operation is idempotent: importing a schema with identical fields, or a config whose values match the latest version, is a no-op and does not create a new version.

```
decree seed <file> [flags]
```

### Options

```
      --auto-publish   auto-publish the schema version
  -h, --help           help for seed
```

### Options inherited from parent commands

```
      --insecure              skip TLS verification (default true)
  -o, --output string         output format: table, json, yaml (default "table")
      --role string           actor role (x-role header) (default "superadmin")
      --server string         gRPC server address (default "localhost:9090")
      --subject string        actor identity (x-subject header)
      --tenant-id string      auth tenant ID (x-tenant-id header)
      --token string          JWT bearer token
      --wait                  wait for the server to be ready before executing the command
      --wait-timeout string   maximum time to wait for server readiness (default "60s")
```

### SEE ALSO

* [decree](decree.md)	 - OpenDecree CLI

