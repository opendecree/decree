# OpenDecree Helm Chart

Deploy OpenDecree to Kubernetes.

## Quick Start

```bash
# In-memory mode (no external deps)
helm install decree deploy/helm/decree \
  --set config.storageBackend=memory

# With PostgreSQL and Redis
helm install decree deploy/helm/decree \
  --set database.existingSecret=db-creds \
  --set redis.existingSecret=redis-creds
```

## Configuration

See [values.yaml](values.yaml) for all options. Key settings:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.storageBackend` | `postgres` or `memory` | `postgres` |
| `config.grpcPort` | gRPC port | `9090` |
| `config.httpPort` | REST gateway port (empty=disabled) | `8080` |
| `config.enableServices` | Comma-separated services | `schema,config,audit` |
| `database.existingSecret` | Secret with DB_WRITE_URL/DB_READ_URL | `""` |
| `redis.existingSecret` | Secret with REDIS_URL | `""` |
| `auth.jwksUrl` | JWKS URL for JWT auth | `""` (metadata auth) |
| `ingress.enabled` | Enable Ingress | `false` |
| `otel.enabled` | Enable OpenTelemetry | `false` |
| `image.pullPolicy` | Defaults to `Always` so security-patch updates on a moving tag propagate; set to `IfNotPresent` only when pinning by digest | `Always` |
| `resources.requests` / `resources.limits` | Default `100m / 128Mi` requests, `1 / 512Mi` limits — override (or set `resources: {}`) for benchmarking, dev, or larger sizing | sane defaults |
| `networkPolicy.enabled` | Restrict ingress + egress to documented dependencies. Off by default in alpha; recommended for any multi-tenant or production cluster. See `networkPolicy.egress.*CIDR` to whitelist PG, Redis, JWKS, OTel | `false` |

## Transport Security (TLS)

The server requires either TLS or insecure mode to start. Use `tls.insecure: true` only in local or dev clusters where encryption is handled externally (e.g. a service mesh or load balancer). Never use insecure mode in production.

**Production example** (server cert + mTLS client CA):

```bash
helm install decree deploy/helm/decree \
  --set tls.insecure=false \
  --set tls.certSecret=decree-tls \
  --set tls.clientCASecret=decree-client-ca
```

**Dev / insecure example** (no encryption):

```bash
helm install decree deploy/helm/decree \
  --set tls.insecure=true
```

| Parameter | Description | Default |
|-----------|-------------|---------|
| `tls.insecure` | Disable gRPC encryption. **Never use in production.** | `false` |
| `tls.certSecret` | Kubernetes TLS Secret (`kubernetes.io/tls`) with `tls.crt` + `tls.key` for the gRPC server. Required when `insecure` is false. | `""` |
| `tls.clientCASecret` | Secret with `ca.crt` for mTLS client verification. | `""` |
| `tls.gateway.caSecret` | CA bundle Secret the HTTP→gRPC gateway uses to verify the upstream gRPC server cert. Defaults to system roots when empty. | `""` |
| `tls.gateway.serverName` | SNI hostname the gateway expects on the upstream gRPC certificate. | `""` |
| `tls.gateway.clientCertSecret` | Secret with `tls.crt` + `tls.key` for the gateway to present as a client cert (mTLS). | `""` |

## Rate Limiting

Rate limiting is enabled by default and applies three role-based token buckets (anonymous, authenticated, superadmin).

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rateLimit.enabled` | Master switch. Set to `false` to disable all rate limiting. | `true` |
| `rateLimit.anonRPS` | Requests per second for unauthenticated callers — shared global bucket per method. | `10` |
| `rateLimit.authedRPS` | Requests per second per tenant for authenticated callers. | `100` |
| `rateLimit.superadminRPS` | Requests per second per superadmin identity. `0` = unlimited. | `0` |
| `rateLimit.burst` | Token bucket burst size (applies to all role classes). | `10` |

## Production hardening checklist

- Pin `image.tag` to an immutable digest (`sha256:…`) and switch `image.pullPolicy` to `IfNotPresent`.
- Override `resources.requests` / `resources.limits` to match your traffic profile.
- Enable `networkPolicy.enabled=true` and populate the `networkPolicy.egress.*CIDR` keys for PostgreSQL, Redis, the JWKS endpoint, and (if used) the OTel collector.
- Use `database.existingSecret` / `redis.existingSecret` instead of plaintext URLs in values.
- Set `tls.insecure=false` and provide `tls.certSecret` with a valid TLS certificate.
