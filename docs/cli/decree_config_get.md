---
title: decree config get
---

## decree config get

Get a single config value

```
decree config get <tenant-id> <field-path> [flags]
```

### Options

```
  -h, --help   help for get
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

