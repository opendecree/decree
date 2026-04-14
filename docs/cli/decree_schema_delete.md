---
title: decree schema delete
---

## decree schema delete

Delete a schema and all its versions (cascades to tenants)

```
decree schema delete <schema-id> [flags]
```

### Options

```
  -h, --help   help for delete
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

* [decree schema](decree_schema.md)	 - Manage configuration schemas

