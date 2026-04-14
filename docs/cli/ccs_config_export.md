---
title: ccs config export
---

## ccs config export

Export config to YAML

```
ccs config export <tenant-id> [flags]
```

### Options

```
  -h, --help            help for export
      --version int32   specific version (default: latest)
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

