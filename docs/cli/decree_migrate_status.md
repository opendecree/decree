---
title: decree migrate status
---

## decree migrate status

Show which migrations have been applied

```
decree migrate status [flags]
```

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
      --db-url string         PostgreSQL connection URL for the owner/superuser role (defaults to $DB_WRITE_URL)
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

* [decree migrate](decree_migrate.md)	 - Apply database migrations

