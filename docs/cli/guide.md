---
title: CLI Guide
---

# CLI Guide

The `decree` CLI manages schemas, tenants, and configuration values against a
running OpenDecree server over gRPC. This guide covers connecting to a server,
authentication, the environment variables that set CLI defaults, and a couple of
worked examples.

> **Alpha Software** — OpenDecree is under active development. CLI commands,
> flags, and environment variables may change without notice between versions.
> Not recommended for production use yet.

For the auto-generated, per-command reference (every flag, every subcommand)
start at [`decree`](decree.md). For the authentication model in depth, see
[Authentication](../concepts/auth.md).

## Installing

```
go install github.com/opendecree/decree/cmd/decree@v0.12.0-alpha.5
```

Verify the install and print the version:

```
decree version
```

## Connecting to a server

By default the CLI talks to `localhost:9090` over TLS. For local development
against a plaintext server, pass `--insecure`:

```
decree --server localhost:9090 --insecure version
```

Every command accepts the global connection flags shown in
[`decree`](decree.md). To avoid repeating them, set the environment variables
below.

## Authentication

OpenDecree authenticates in one of two modes; both are driven from the CLI by
flags or environment variables.

- **Metadata headers (default).** The CLI sends the actor identity, role, and
  tenant as gRPC metadata headers via `--subject`, `--role`, and `--tenant-id`.
  The CLI defaults `--role` to `superadmin` for convenience; the *server*
  defaults a missing role to `user`.
- **JWT bearer token (opt-in).** When the server is configured for JWT/JWKS,
  pass a token with `--token` or, preferably, `--token-file`. Prefer the
  `DECREE_TOKEN` environment variable or `--token-file` over `--token` so the
  token does not land in your shell history.

```
# Metadata-header auth (default mode)
decree --subject alice --role admin --tenant-id acme config get-all acme

# JWT auth — read the token from a file to keep it out of shell history
decree --token-file ~/.decree/token config get-all acme
```

See [Authentication](../concepts/auth.md) for how the server validates these
credentials.

## Environment variables

Each global connection flag has a matching `DECREE_*` environment variable.
The flag, when passed, takes precedence over the environment variable.

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `DECREE_SERVER` | `--server` | `localhost:9090` | gRPC server address |
| `DECREE_SUBJECT` | `--subject` | _(empty)_ | Actor identity (`x-subject` header) |
| `DECREE_ROLE` | `--role` | `superadmin` | Actor role (`x-role` header) |
| `DECREE_TENANT_ID` | `--tenant-id` | _(empty)_ | Auth tenant ID (`x-tenant-id` header) |
| `DECREE_TOKEN` | `--token` | _(empty)_ | JWT bearer token (prefer this over `--token` to avoid shell-history exposure) |
| `DECREE_INSECURE` | `--insecure` | `false` | Disable TLS (plaintext); local development only |
| `DECREE_WAIT_TIMEOUT` | `--wait-timeout` | `60s` | Maximum time to wait for server readiness (with `--wait`) |

Setting these once in your shell removes the need to repeat the flags:

```
export DECREE_SERVER=localhost:9090
export DECREE_INSECURE=true
export DECREE_SUBJECT=alice
export DECREE_ROLE=admin
export DECREE_TENANT_ID=acme

decree config get-all acme   # uses the exported defaults
```

> **Note** — `--token-file` is a flag only; it has no `DECREE_*` environment
> variable. Point it at a file containing the JWT; it takes precedence over
> `--token` / `DECREE_TOKEN`.

## Worked examples

The examples below assume the connection and auth defaults are exported as shown
above.

### Set and get a config value

```
# Set a single value (parsed according to the schema field's type)
decree config set acme features.dark_mode true

# Read it back
decree config get acme features.dark_mode

# Read the whole tenant config as JSON
decree config get-all acme --output json
```

### Bootstrap a tenant from a YAML file

`decree seed` creates a schema, tenant, and/or config in one step from a single
YAML file — handy for getting a new environment running:

```
decree seed -f ./bootstrap.yaml
```

### Wait for the server before running

In scripts and CI, use `--wait` so the command blocks until the server is ready
instead of failing fast:

```
decree --wait --wait-timeout 30s version
```

## Shell completion

Generate a completion script for your shell with `decree completion`:

```
# zsh — load completions for the current session
source <(decree completion zsh)
```

See [`decree completion`](decree_completion.md) for `bash`, `fish`, and
`powershell`, plus instructions for installing the script permanently.
