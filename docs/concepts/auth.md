# Authentication & Authorization

OpenDecree supports two authentication modes: **metadata headers** (the default) and **JWT** (opt-in). Authorization uses three roles with field-level locking for fine-grained control.

## Two Auth Modes

### Metadata Headers (Default)

By default, OpenDecree trusts identity from gRPC metadata headers. This is designed for development, internal services behind a trusted gateway, or environments where auth is handled upstream.

| Header | Required | Description |
|--------|----------|-------------|
| `x-subject` | Yes | Actor identity (e.g., `admin@example.com`) |
| `x-role` | No | `superadmin`, `admin`, or `user` (default `user`) |
| `x-tenant-id` | Conditional | Required for `admin` and `user` roles |

When `x-role` is omitted, the request defaults to `user` -- the least-privileged role. This is the safe default: a caller that forgets to set a role gets read-only access, not full control. Because `user` is tenant-scoped, a request that sends **only** `x-subject` (no `x-role`, no `x-tenant-id`) is **rejected** with `PermissionDenied` (`x-tenant-id required for non-superadmin`).

To act as a superadmin you must set `x-role: superadmin` explicitly.

> **Restoring the old default (discouraged).** Earlier versions defaulted a missing `x-role` to `superadmin`. To restore that behaviour during a migration window, set `DECREE_INSECURE_DEFAULT_SUPERADMIN=1` on the server. This is insecure -- any client that omits `x-role` is silently granted superadmin -- and logs a `WARN` on startup and on every request that uses the fallback. Never enable it in production; remove it once callers send explicit roles. See [Server Configuration](../server/configuration.md#authentication) and the [Migration Guide](../../MIGRATION.md).

With the CLI, local development stays frictionless because the `decree` CLI defaults `--role` to `superadmin` (overridable via the `--role` flag or `DECREE_ROLE` env var) -- so a superadmin role is sent for you. This is a CLI default, not a server default; raw gRPC clients get `user` unless they set `x-role` themselves.

```bash
export DECREE_SUBJECT=admin@example.com    # x-subject
export DECREE_ROLE=admin                   # x-role (optional; CLI default is superadmin)
export DECREE_TENANT_ID=<tenant-id>        # x-tenant-id (required for admin/user)
```

### JWT (Opt-in)

Enable JWT validation by setting the `JWT_JWKS_URL` environment variable. When enabled, OpenDecree validates the JWT token from the `authorization` header against the JWKS endpoint.

```bash
JWT_JWKS_URL=https://auth.example.com/.well-known/jwks.json
JWT_ISSUER=https://auth.example.com    # optional — validates iss claim
```

The server extracts `subject`, `role`, and `tenant_id` from JWT claims instead of metadata headers. The exact claim mapping depends on your identity provider configuration.

## Three Roles

| Role | Scope | Can do |
|------|-------|--------|
| `superadmin` | Global | Everything. No tenant restriction. Bypasses field locks. |
| `admin` | Single tenant | Read/write config, manage field locks. Bound to `x-tenant-id`. |
| `user` | Single tenant | Read config only. Bound to `x-tenant-id`. |

Key rules:

- **superadmin** does not need a tenant ID -- it can operate on any tenant
- **admin** and **user** must provide a tenant ID and can only access that tenant's data
- Schema management (create, update, publish) requires `superadmin`
- Tenant creation requires `superadmin`
- Audit queries follow the same tenant scoping

## Field Locks

Field locks prevent specific config fields from being modified by non-superadmin users. This is useful for protecting critical settings that should only be changed through a controlled process.

```bash
# Lock a field — only superadmin can change it
decree lock set <tenant-id> payments.currency

# Unlock it
decree lock remove <tenant-id> payments.currency

# List all locks for a tenant
decree lock list <tenant-id>
```

When an `admin` tries to write to a locked field, the server returns a `PermissionDenied` error. Superadmins bypass all field locks.

### Enum Value Locks

For enum fields, you can lock specific values rather than the entire field:

```bash
# Lock specific enum values — admin cannot set currency to GBP or JPY
decree lock set <tenant-id> payments.currency --values GBP,JPY
```

The admin can still change the field to other allowed enum values (e.g., USD, EUR) but cannot select locked values.

## Configuring Auth

### For Local Development

No server configuration needed -- metadata auth is the default. The `decree` CLI sends `x-role: superadmin` by default (see [Two Auth Modes](#metadata-headers-default) above), so you only need to set a subject:

```bash
export DECREE_SUBJECT=dev@example.com
decree config get-all <tenant-id>
```

A raw gRPC client gets the `user` role unless it sets `x-role`, so it must also send `x-tenant-id` (or set `x-role: superadmin`) to avoid a `PermissionDenied` rejection.

### For Staging / Internal Services

Use metadata headers with a gateway that sets the headers based on upstream auth:

```yaml
# No JWT env vars — metadata mode
environment:
  GRPC_PORT: "9090"
  DB_WRITE_URL: "postgres://..."
  REDIS_URL: "redis://..."
```

### For Production with JWT

```yaml
environment:
  JWT_JWKS_URL: "https://auth.example.com/.well-known/jwks.json"
  JWT_ISSUER: "https://auth.example.com"
```

## Pluggable Guard Layer

Authorization checks are centralised in the `internal/authz` package as a composable guard interface:

```go
type Guard interface {
    Check(ctx context.Context, action Action, resource Resource) error
}
```

Three built-in guards compose the default chain:

| Guard | What it checks |
|-------|---------------|
| `TenantScopeGuard` | Caller has access to `resource.TenantID` |
| `RolePolicyGuard` | `ActionWrite` → admin+; `ActionAdmin` → superadmin; `ActionRead` → pass |
| `FieldLockGuard` | Write to `resource.FieldPath` is not locked (config service only) |

`ChainGuard` runs them in order, stopping on the first error. To add a new authorization axis (e.g. rate-limit-per-field, IP allowlist), implement `Guard` and pass a new chain via `WithGuard(authz.Chain(...))` to the relevant service constructor.

## Related

- [Server Configuration](../server/configuration.md) -- all auth-related environment variables
- [Tenants](tenants.md) -- how tenant scoping works
- [API Reference](../api/api-reference.md) -- RPC-level auth requirements
- [CLI Reference](../cli/decree.md) — CLI auth flags and environment variables
