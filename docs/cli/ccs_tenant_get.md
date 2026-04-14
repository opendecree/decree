---
title: ccs tenant get
---

## ccs tenant get

Show a tenant

```
ccs tenant get <tenant-id> [flags]
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

* [ccs tenant](ccs_tenant.md)	 - Manage tenants

