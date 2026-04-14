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
