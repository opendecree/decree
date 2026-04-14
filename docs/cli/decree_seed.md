---
title: decree seed
---

## decree seed

Bootstrap a schema, tenant, and config from a single YAML file

### Synopsis

Seed creates a schema, tenant, and initial configuration from a single YAML file. The operation is idempotent: existing schemas with identical fields are skipped, existing tenants are reused, and config values are merged.

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
      --insecure           skip TLS verification (default true)
  -o, --output string      output format: table, json, yaml (default "table")
      --role string        actor role (x-role header) (default "superadmin")
      --server string      gRPC server address (default "localhost:9090")
      --subject string     actor identity (x-subject header)
      --tenant-id string   auth tenant ID (x-tenant-id header)
      --token string       JWT bearer token
```

### SEE ALSO

* [decree](decree.md)	 - OpenDecree CLI

