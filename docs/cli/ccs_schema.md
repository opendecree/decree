---
title: ccs schema
---

## ccs schema

Manage configuration schemas

### Options

```
  -h, --help   help for schema
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
* [ccs schema create](ccs_schema_create.md)	 - Create a new schema from a YAML file
* [ccs schema delete](ccs_schema_delete.md)	 - Delete a schema and all its versions (cascades to tenants)
* [ccs schema export](ccs_schema_export.md)	 - Export a schema to YAML
* [ccs schema get](ccs_schema_get.md)	 - Show a schema
* [ccs schema import](ccs_schema_import.md)	 - Import a schema from a YAML file
* [ccs schema list](ccs_schema_list.md)	 - List all schemas
* [ccs schema publish](ccs_schema_publish.md)	 - Publish a schema version (makes it immutable and assignable to tenants)

