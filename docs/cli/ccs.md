---
title: ccs
---

## ccs

Central Config Service CLI

### Synopsis

Command-line tool for managing schemas, tenants, and configuration values in the Central Config Service.

### Options

```
  -h, --help               help for ccs
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
* [ccs completion](ccs_completion.md)	 - Generate the autocompletion script for the specified shell
* [ccs config](ccs_config.md)	 - Read and write configuration values
* [ccs lock](ccs_lock.md)	 - Manage field locks
* [ccs schema](ccs_schema.md)	 - Manage configuration schemas
* [ccs tenant](ccs_tenant.md)	 - Manage tenants
* [ccs watch](ccs_watch.md)	 - Stream live config changes (like tail -f)

