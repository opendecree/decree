---
title: decree docgen
---

## decree docgen

Generate markdown documentation from a schema

### Synopsis

Generate markdown documentation from a schema. Provide a schema-id to fetch from the server, or --file to use a local YAML file.

```
decree docgen [schema-id] [flags]
```

### Options

```
      --file string          schema YAML file (offline mode)
  -h, --help                 help for docgen
      --no-constraints       omit constraint details
      --no-deprecated        exclude deprecated fields
      --no-grouping          flat list instead of grouped by prefix
      --output-file string   write output to file instead of stdout
      --version int32        schema version (default: latest)
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

