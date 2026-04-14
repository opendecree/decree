---
title: decree audit
---

## decree audit

Query audit logs and usage statistics

### Synopsis

Query the configuration change history and field read usage statistics. Useful for compliance auditing and identifying unused fields.

### Options

```
  -h, --help   help for audit
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

* [decree](decree.md)	 - OpenDecree CLI
* [decree audit query](decree_audit_query.md)	 - Query the config change audit log
* [decree audit unused](decree_audit_unused.md)	 - Find fields not read since the given duration (e.g. 7d, 24h)
* [decree audit usage](decree_audit_usage.md)	 - Show read usage statistics for a field

