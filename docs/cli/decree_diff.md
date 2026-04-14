---
title: decree diff
---

## decree diff

Show differences between two config versions or files

### Synopsis

Compare two configuration snapshots and show the differences.

Server mode (compare two versions of a tenant's config):
  decree diff <tenant-id> <version-a> <version-b>

File mode (compare two local config YAML files):
  decree diff --old config-v1.yaml --new config-v2.yaml

```
decree diff [flags]
```

### Options

```
  -h, --help         help for diff
      --new string   new config YAML file (file mode)
      --old string   old config YAML file (file mode)
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

