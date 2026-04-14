---
title: decree config get-all
---

## decree config get-all

Get all config values for a tenant

```
decree config get-all <tenant-id> [flags]
```

### Options

```
  -h, --help   help for get-all
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

* [decree config](decree_config.md)	 - Read and write configuration values

