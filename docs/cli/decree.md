---
title: decree
---

## decree

OpenDecree CLI

### Synopsis

Command-line tool for managing schemas, tenants, and configuration values in the OpenDecree.

### Options

```
  -h, --help               help for decree
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
* [decree completion](decree_completion.md)	 - Generate the autocompletion script for the specified shell
* [decree config](decree_config.md)	 - Read and write configuration values
* [decree lock](decree_lock.md)	 - Manage field locks
* [decree schema](decree_schema.md)	 - Manage configuration schemas
* [decree tenant](decree_tenant.md)	 - Manage tenants
* [decree version](decree_version.md)	 - Print the CLI version
* [decree watch](decree_watch.md)	 - Stream live config changes (like tail -f)

