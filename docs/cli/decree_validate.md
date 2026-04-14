---
title: decree validate
---

## decree validate

Validate a config YAML against a schema YAML (offline)

```
decree validate [flags]
```

### Options

```
      --config string   config YAML file
  -h, --help            help for validate
      --schema string   schema YAML file
      --strict          reject unknown fields not in schema
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

