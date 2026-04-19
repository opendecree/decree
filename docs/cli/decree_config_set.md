---
title: decree config set
---

## decree config set

Set a single config value

### Synopsis

Set a single config value.

Values are parsed according to the schema's field type:
  string    -> as-is
  integer   -> decimal integer (e.g. 42)
  number    -> float (e.g. 3.14)
  bool      -> true / false
  time      -> RFC3339 (e.g. 2006-01-02T15:04:05Z)
  duration  -> Go duration (e.g. 15s, 2h, 500ms)
  url       -> as-is
  json      -> must be valid JSON

```
decree config set <tenant-id> <field-path> <value> [flags]
```

### Options

```
  -h, --help   help for set
```

### Options inherited from parent commands

```
      --insecure              skip TLS verification (default true)
  -o, --output string         output format: table, json, yaml (default "table")
      --role string           actor role (x-role header) (default "superadmin")
      --server string         gRPC server address (default "localhost:9090")
      --subject string        actor identity (x-subject header)
      --tenant-id string      auth tenant ID (x-tenant-id header)
      --token string          JWT bearer token
      --wait                  wait for the server to be ready before executing the command
      --wait-timeout string   maximum time to wait for server readiness (default "60s")
```

### SEE ALSO

* [decree config](decree_config.md)	 - Read and write configuration values

