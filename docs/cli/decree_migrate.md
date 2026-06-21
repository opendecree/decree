---
title: decree migrate
---

## decree migrate

Apply database migrations

### Synopsis

Apply the OpenDecree database schema migrations.

Migrations create the tables, row-level-security policies, and the unprivileged
"decree_app" role that the server assumes (SET ROLE) on every connection. A fresh
database must be migrated before the server can start, otherwise the server fails
with: role "decree_app" does not exist.

Connect as a role that may CREATE ROLE and GRANT — the database owner or a
superuser — not as decree_app. Running "up" repeatedly is safe; already-applied
migrations are skipped.

### Options

```
      --db-url string   PostgreSQL connection URL for the owner/superuser role (defaults to $DB_WRITE_URL)
  -h, --help            help for migrate
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

* [decree](decree.md)	 - OpenDecree CLI
* [decree migrate down](decree_migrate_down.md)	 - Roll back the most recent migration
* [decree migrate status](decree_migrate_status.md)	 - Show which migrations have been applied
* [decree migrate up](decree_migrate_up.md)	 - Apply all pending migrations

