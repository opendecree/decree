---
title: decree lock remove
---

## decree lock remove

Unlock a field

```
decree lock remove <tenant-id> <field-path> [flags]
```

### Options

```
  -h, --help   help for remove
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

* [decree lock](decree_lock.md)	 - Manage field locks

