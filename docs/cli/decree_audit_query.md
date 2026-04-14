---
title: decree audit query
---

## decree audit query

Query the config change audit log

```
decree audit query [flags]
```

### Options

```
      --actor string    filter by actor
      --field string    filter by field path
  -h, --help            help for query
      --since string    show entries from the last duration (e.g. 24h, 7d)
      --tenant string   filter by tenant ID
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

* [decree audit](decree_audit.md)	 - Query audit logs and usage statistics

