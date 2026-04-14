---
title: ccs lock list
---

## ccs lock list

List field locks for a tenant

```
ccs lock list <tenant-id> [flags]
```

### Options

```
  -h, --help   help for list
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

* [ccs lock](ccs_lock.md)	 - Manage field locks

