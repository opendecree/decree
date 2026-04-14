---
title: ccs tenant
---

## ccs tenant

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

* [ccs](ccs.md)	 - Central Config Service CLI
* [ccs tenant create](ccs_tenant_create.md)	 - Create a new tenant on a published schema version
* [ccs tenant delete](ccs_tenant_delete.md)	 - Delete a tenant and all its configuration data
* [ccs tenant get](ccs_tenant_get.md)	 - Show a tenant
* [ccs tenant list](ccs_tenant_list.md)	 - List tenants

