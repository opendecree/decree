---
title: ccs audit unused
---

## ccs audit unused

Find fields not read since the given duration (e.g. 7d, 24h)

```
ccs audit unused <tenant-id> <since-duration> [flags]
```

### Options

```
  -h, --help   help for unused
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

* [ccs audit](ccs_audit.md)	 - Query audit logs and usage statistics

