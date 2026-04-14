---
title: ccs config import
---

## ccs config import

Import config from a YAML file

```
ccs config import <tenant-id> <file> [flags]
```

### Options

```
      --description string   version description
  -h, --help                 help for import
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

* [ccs config](ccs_config.md)	 - Read and write configuration values

