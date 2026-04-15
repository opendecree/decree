---
title: decree config import
---

## decree config import

Import config from a YAML file

```
decree config import <tenant-id> <file> [flags]
```

### Options

```
      --description string   version description
  -h, --help                 help for import
      --mode string          import mode: merge, replace, or defaults (default "merge")
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

* [decree config](decree_config.md)	 - Read and write configuration values

