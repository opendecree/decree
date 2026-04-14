---
title: decree
---

## decree

OpenDecree CLI

### Synopsis

Command-line tool for managing schemas, tenants, and configuration values in OpenDecree.

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
* [decree diff](decree_diff.md)	 - Show differences between two config versions or files
* [decree docgen](decree_docgen.md)	 - Generate markdown documentation from a schema
* [decree dump](decree_dump.md)	 - Export a full tenant backup (schema + config + locks)
* [decree lock](decree_lock.md)	 - Manage field locks
* [decree schema](decree_schema.md)	 - Manage configuration schemas
* [decree seed](decree_seed.md)	 - Bootstrap a schema, tenant, and config from a single YAML file
* [decree tenant](decree_tenant.md)	 - Manage tenants
* [decree validate](decree_validate.md)	 - Validate a config YAML against a schema YAML (offline)
* [decree version](decree_version.md)	 - Print the CLI version
* [decree watch](decree_watch.md)	 - Stream live config changes (like tail -f)

