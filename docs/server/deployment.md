# Deployment

OpenDecree is a single Go binary with two external dependencies: PostgreSQL and Redis (or zero dependencies in memory mode). This page covers local development, Docker, Helm, and raw Kubernetes deployment.

## Quick Start (No Docker)

Run with in-memory storage — zero external dependencies:

```bash
# go install names the server binary "server" after its directory; the canonical
# name is decree-server, matching the release archives and Docker image.
go install github.com/opendecree/decree/cmd/server@latest
mv "$(go env GOPATH)/bin/server" "$(go env GOPATH)/bin/decree-server"
STORAGE_BACKEND=memory HTTP_PORT=8080 decree-server

# Swagger UI: http://localhost:8080/docs
# gRPC: localhost:9090
```

## Docker Compose (Local Development)

The repository includes a `docker-compose.yml` that starts the full stack:

```bash
git clone https://github.com/opendecree/decree.git
cd decree

# Start everything: PostgreSQL, Redis, migrations, and the service.
# The first run builds the server and migration images from source
# (a few minutes); later runs reuse the cached images.
docker compose up -d --wait service
```

This starts:

| Service | Port | Purpose |
|---------|------|---------|
| `postgres` | 5432 | PostgreSQL 17 database |
| `redis` | 6379 | Redis 7 for cache + pub/sub |
| `migrate` | -- | Runs goose migrations, then exits |
| `service` | 9090 (gRPC), 8080 (HTTP) | OpenDecree server |

The service is ready when `docker compose up --wait` returns. No JWT configuration needed — metadata auth is the default.

### Adding Observability

To include tracing and metrics, start the observability stack alongside the service:

```bash
docker compose up -d --wait service otel-collector jaeger
```

This adds:

| Service | Port | Purpose |
|---------|------|---------|
| `otel-collector` | 4317 (gRPC), 4318 (HTTP) | OpenTelemetry Collector |
| `jaeger` | 16686 | Jaeger UI for viewing traces |

Then enable OTel on the service by adding environment variables. See [Observability](observability.md) for details.

### Tearing Down

```bash
docker compose down        # stop containers
docker compose down -v     # stop containers and delete volumes (database data)
```

## Docker Image

The repository includes a multi-stage Dockerfile at `build/Dockerfile`:

```bash
docker build -f build/Dockerfile -t decree:latest .

# Run with in-memory storage
docker run -p 9090:9090 -p 8080:8080 \
  -e STORAGE_BACKEND=memory -e HTTP_PORT=8080 \
  decree:latest

# Run with PostgreSQL + Redis
docker run -p 9090:9090 -p 8080:8080 \
  -e DB_WRITE_URL=postgres://... \
  -e REDIS_URL=redis://... \
  -e HTTP_PORT=8080 \
  decree:latest
```

Pre-built images are available on ghcr.io:

```bash
docker pull ghcr.io/opendecree/decree:latest       # server
docker pull ghcr.io/opendecree/decree-cli:latest    # CLI
```

## Helm Chart

A Helm chart is provided at `deploy/helm/decree/`.

### Quick Install

```bash
# In-memory mode (no external deps — good for evaluation)
helm install decree deploy/helm/decree \
  --set config.storageBackend=memory

# With PostgreSQL and Redis (production)
helm install decree deploy/helm/decree \
  --set database.existingSecret=db-creds \
  --set redis.existingSecret=redis-creds
```

### Key Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.storageBackend` | `postgres` or `memory` | `postgres` |
| `config.grpcPort` | gRPC port | `9090` |
| `config.httpPort` | REST gateway port (empty = disabled) | `8080` |
| `config.enableServices` | Comma-separated services | `schema,config,audit` |
| `database.existingSecret` | Secret with DB_WRITE_URL / DB_READ_URL | `""` |
| `redis.existingSecret` | Secret with REDIS_URL | `""` |
| `auth.jwksUrl` | JWKS URL for JWT auth | `""` (metadata auth) |
| `ingress.enabled` | Enable Ingress | `false` |
| `otel.enabled` | Enable OpenTelemetry | `false` |
| `resources` | CPU/memory limits | `{}` |
| `replicaCount` | Number of replicas | `1` |

See [`deploy/helm/decree/values.yaml`](https://github.com/opendecree/decree/blob/main/deploy/helm/decree/values.yaml) for all options.

### Split Deployments

Use `config.enableServices` to run specialized instances:

```bash
# High-traffic config reads
helm install decree-config deploy/helm/decree \
  --set config.enableServices=config \
  --set replicaCount=3

# Admin operations (lower traffic)
helm install decree-admin deploy/helm/decree \
  --set config.enableServices="schema,audit"
```

## Database Migrations

OpenDecree uses [goose](https://github.com/pressly/goose) for database migrations. Migrations live in `db/migrations/`.

The server does **not** migrate on startup: it assumes the unprivileged `decree_app` role (created by the baseline migration) on every connection, so a fresh database must be migrated **before** the server starts — otherwise it fails with `role "decree_app" does not exist`.

### Using the `decree migrate` command (recommended)

The `decree` CLI bundles the migrations, so the `decree-cli` image is a turnkey migration runner — no source checkout or SQL mount required:

```bash
docker run --rm \
  -e DB_WRITE_URL="postgres://owner:pass@host:5432/centralconfig?sslmode=disable" \
  ghcr.io/opendecree/decree-cli:latest migrate up
```

Connect as the database **owner/superuser** — the migrations create the `decree_app` role and its grants, so they cannot run as `decree_app` itself. `migrate up` is idempotent; use `migrate status` to inspect applied versions. In Kubernetes, run it as a `pre-install,pre-upgrade` Helm hook Job (or an init container) so it completes before the server starts.

### Running Migrations Manually (goose)

```bash
# Using the tools Docker image
docker build -f build/Dockerfile.tools -t decree-tools:latest .
docker run --rm decree-tools:latest \
  goose -dir /migrations postgres \
  "postgres://user:pass@host:5432/centralconfig?sslmode=disable" up
```

### Migration in Docker Compose

The `migrate` service in `docker-compose.yml` runs migrations automatically before the service starts. It waits for PostgreSQL to be healthy, runs `goose up`, and exits.

## Kubernetes (Raw Manifests)

If you prefer raw manifests over Helm:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: decree
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: decree
          image: ghcr.io/opendecree/decree:latest
          ports:
            - containerPort: 9090
              protocol: TCP
            - containerPort: 8080
              protocol: TCP
          env:
            - name: GRPC_PORT
              value: "9090"
            - name: HTTP_PORT
              value: "8080"
            - name: DB_WRITE_URL
              valueFrom:
                secretKeyRef:
                  name: decree-db
                  key: write-url
            - name: DB_READ_URL
              valueFrom:
                secretKeyRef:
                  name: decree-db
                  key: read-url
            - name: REDIS_URL
              valueFrom:
                secretKeyRef:
                  name: decree-redis
                  key: url
          readinessProbe:
            grpc:
              port: 9090
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            grpc:
              port: 9090
            initialDelaySeconds: 10
            periodSeconds: 30
```

OpenDecree exposes the standard gRPC health checking protocol, so Kubernetes gRPC probes work out of the box.

## HTTP Gateway — Auth Header Security

By default, the HTTP gateway **rejects** any request that carries `x-subject`, `x-role`, or `x-tenant-id` headers. These are the metadata-auth identity headers; allowing clients to set them directly enables impersonation attacks.

If you run a trusted authentication proxy (e.g. an Envoy sidecar, an Istio ingress, or a custom API gateway) in front of the HTTP gateway that sets these headers, you must declare it as trusted:

```bash
DECREE_GATEWAY_TRUSTED_PROXY=1 decree-server
```

A `WARN` is logged at startup when this flag is set. Only enable it if you can guarantee that the proxy strips or overwrites any client-supplied auth headers before they reach the gateway.

> **Note**: The `authorization` header (JWT Bearer tokens) is always forwarded. The restriction only applies to the three metadata-auth headers listed above.

## Health Checks

Each enabled service registers with the gRPC health checking protocol. Services report `SERVING` once fully initialized:

- `centralconfig.v1.SchemaService`
- `centralconfig.v1.ConfigService`
- `centralconfig.v1.AuditService`

Use `grpc-health-probe` or Kubernetes native gRPC probes to check readiness.

## Related

- [Server Configuration](configuration.md) — all environment variables
- [Observability](observability.md) — OTel setup with Docker Compose
- [Getting Started](../getting-started.md) — quick start walkthrough
