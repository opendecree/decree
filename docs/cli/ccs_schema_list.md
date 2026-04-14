---
title: ccs schema list
---

## ccs schema list

List all schemas

```
ccs schema list [flags]
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

* [ccs schema](ccs_schema.md)	 - Manage configuration schemas

