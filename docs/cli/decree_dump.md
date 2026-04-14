---
title: decree dump
---

## decree dump

Export a full tenant backup (schema + config + locks)

### Synopsis

Dump exports a tenant's schema, configuration, and field locks as a single YAML file. The output is seed-compatible and can be used to recreate the tenant elsewhere.

```
decree dump <tenant-id> [flags]
```

### Options

```
  -h, --help                 help for dump
      --no-locks             exclude field locks
      --output-file string   write to file instead of stdout
      --version int32        config version (default: latest)
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

