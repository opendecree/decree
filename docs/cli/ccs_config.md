---
title: decree config
---

## decree config

Read and write configuration values

### Options

```
  -h, --help   help for config
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

* [ccs](ccs.md)	 - Central Config Service CLI
* [decree config export](decree_config_export.md)	 - Export config to YAML
* [decree config get](decree_config_get.md)	 - Get a single config value
* [decree config get-all](decree_config_get-all.md)	 - Get all config values for a tenant
* [decree config import](decree_config_import.md)	 - Import config from a YAML file
* [decree config rollback](decree_config_rollback.md)	 - Rollback config to a previous version
* [decree config set](decree_config_set.md)	 - Set a single config value
* [decree config set-many](decree_config_set-many.md)	 - Set multiple config values atomically
* [decree config versions](decree_config_versions.md)	 - List config versions

