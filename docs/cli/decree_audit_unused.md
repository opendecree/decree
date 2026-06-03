---
title: decree audit unused
---

## decree audit unused

Find fields not read since the given duration (e.g. 7d, 24h)

```
decree audit unused <tenant-id> <since-duration> [flags]
```

### Options

```
  -h, --help   help for unused
```

### Options inherited from parent commands

```
      --insecure              disable TLS (plaintext); for local development only
  -o, --output string         output format: table, json, yaml (default "table")
      --role string           actor role (x-role header) (default "superadmin")
      --server string         gRPC server address (default "localhost:9090")
      --subject string        actor identity (x-subject header)
      --tenant-id string      auth tenant ID (x-tenant-id header)
      --token string          JWT bearer token (prefer DECREE_TOKEN env var to avoid shell history exposure)
      --token-file string     path to a file containing the JWT bearer token (takes precedence over --token)
      --wait                  wait for the server to be ready before executing the command
      --wait-timeout string   maximum time to wait for server readiness (default "60s")
```

### SEE ALSO

* [decree audit](decree_audit.md)	 - Query audit logs and usage statistics

