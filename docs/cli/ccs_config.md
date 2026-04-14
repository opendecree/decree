---
title: ccs config
---

## ccs config

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
* [ccs config export](ccs_config_export.md)	 - Export config to YAML
* [ccs config get](ccs_config_get.md)	 - Get a single config value
* [ccs config get-all](ccs_config_get-all.md)	 - Get all config values for a tenant
* [ccs config import](ccs_config_import.md)	 - Import config from a YAML file
* [ccs config rollback](ccs_config_rollback.md)	 - Rollback config to a previous version
* [ccs config set](ccs_config_set.md)	 - Set a single config value
* [ccs config set-many](ccs_config_set-many.md)	 - Set multiple config values atomically
* [ccs config versions](ccs_config_versions.md)	 - List config versions

