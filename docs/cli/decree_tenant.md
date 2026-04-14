---
title: decree tenant
---

## decree tenant

Manage tenants

### Options

```
  -h, --help   help for tenant
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
* [decree tenant create](decree_tenant_create.md)	 - Create a new tenant on a published schema version
* [decree tenant delete](decree_tenant_delete.md)	 - Delete a tenant and all its configuration data
* [decree tenant get](decree_tenant_get.md)	 - Show a tenant
* [decree tenant list](decree_tenant_list.md)	 - List tenants

