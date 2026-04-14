# Authentication & Authorization

CCS supports two authentication modes: **metadata headers** (the default) and **JWT** (opt-in). Authorization uses three roles with field-level locking for fine-grained control.

## Two Auth Modes

### Metadata Headers (Default)

By default, CCS trusts identity from gRPC metadata headers. This is designed for development, internal services behind a trusted gateway, or environments where auth is handled upstream.

| Header | Required | Description |
|--------|----------|-------------|
| `x-subject` | Yes | Actor identity (e.g., `admin@example.com`) |
| `x-role` | No | `superadmin` (default), `admin`, or `user` |
| `x-tenant-id` | Conditional | Required for `admin` and `user` roles |

When `x-role` is omitted, the request defaults to `superadmin`. This makes local development frictionless -- you only need to set `x-subject`.

With the CLI:

```bash
export DECREE_SUBJECT=admin@example.com    # x-subject
export DECREE_ROLE=admin                   # x-role (optional)
export DECREE_TENANT=<tenant-id>           # x-tenant-id (optional)
```

### JWT (Opt-in)

Enable JWT validation by setting the `JWT_JWKS_URL` environment variable. When enabled, CCS validates the JWT token from the `authorization` header against the JWKS endpoint.

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

No configuration needed. Metadata auth is the default:

```bash
<<<<<<< HEAD
export DECREE_SUBJECT=dev@example.com
=======
export CCS_SUBJECT=dev@example.com
>>>>>>> origin/main
decree config get-all <tenant-id>
```

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

## Related

- [Server Configuration](../server/configuration.md) -- all auth-related environment variables
- [Tenants](tenants.md) -- how tenant scoping works
- [API Reference](../api/api-reference.md) -- RPC-level auth requirements
- [CLI Reference](../cli/ccs.md) -- CLI auth flags and environment variables
