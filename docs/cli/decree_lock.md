---
title: decree lock
---

## decree lock

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

<<<<<<< HEAD:docs/cli/decree_lock.md
* [decree](decree.md)	 - OpenDecree CLI
=======
* [ccs](ccs.md)	 - Central Config Service CLI
>>>>>>> origin/main:docs/cli/ccs_lock.md
* [decree lock list](decree_lock_list.md)	 - List field locks for a tenant
* [decree lock remove](decree_lock_remove.md)	 - Unlock a field
* [decree lock set](decree_lock_set.md)	 - Lock a field (prevents modification by non-superadmin)

