---
title: decree audit verify
---

## decree audit verify

Verify the tamper-evident audit chain for a tenant

### Synopsis

Fetches all audit entries for the tenant (or the global chain if --tenant is
omitted), recomputes each entry_hash, and reports any breaks.

Requires the server's database schema to be up to date so that the tamper-evident
hash columns are populated.

```
decree audit verify [flags]
```

### Options

```
  -h, --help            help for verify
      --tenant string   tenant ID to verify (empty = global chain)
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

