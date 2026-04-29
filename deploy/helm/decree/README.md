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

## Production hardening checklist

- Pin `image.tag` to an immutable digest (`sha256:…`) and switch `image.pullPolicy` to `IfNotPresent`.
- Override `resources.requests` / `resources.limits` to match your traffic profile.
- Enable `networkPolicy.enabled=true` and populate the `networkPolicy.egress.*CIDR` keys for PostgreSQL, Redis, the JWKS endpoint, and (if used) the OTel collector.
- Use `database.existingSecret` / `redis.existingSecret` instead of plaintext URLs in values.
