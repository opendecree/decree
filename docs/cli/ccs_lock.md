---
title: ccs lock
---

## ccs lock

Manage field locks

### Options

```
  -h, --help   help for lock
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

* [ccs](ccs.md)	 - Central Config Service CLI
* [ccs lock list](ccs_lock_list.md)	 - List field locks for a tenant
* [ccs lock remove](ccs_lock_remove.md)	 - Unlock a field
* [ccs lock set](ccs_lock_set.md)	 - Lock a field (prevents modification by non-superadmin)

