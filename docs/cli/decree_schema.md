---
title: decree schema
---

## decree schema

Manage configuration schemas

### Synopsis

Create, list, publish, import/export, and delete configuration schemas. Schemas define the allowed fields, types, and constraints for tenant configurations.

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

* [decree](decree.md)	 - OpenDecree CLI
* [decree schema create](decree_schema_create.md)	 - Create a new schema from a YAML file
* [decree schema delete](decree_schema_delete.md)	 - Delete a schema and all its versions (cascades to tenants)
* [decree schema export](decree_schema_export.md)	 - Export a schema to YAML
* [decree schema get](decree_schema_get.md)	 - Show a schema
* [decree schema import](decree_schema_import.md)	 - Import a schema from a YAML file
* [decree schema list](decree_schema_list.md)	 - List all schemas
* [decree schema publish](decree_schema_publish.md)	 - Publish a schema version (makes it immutable and assignable to tenants)

