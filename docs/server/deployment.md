# Deployment

CCS is a single Go binary with two external dependencies: PostgreSQL and Redis. This page covers local development with Docker Compose, building the Docker image, and Kubernetes deployment.

## Docker Compose (Local Development)

The repository includes a `docker-compose.yml` that starts the full stack:

```bash
git clone https://github.com/zeevdr/central-config-service.git
cd central-config-service

# Start everything: PostgreSQL, Redis, migrations, and the service
docker compose up -d --wait service
```

This starts:

| Service | Port | Purpose |
|---------|------|---------|
| `postgres` | 5432 | PostgreSQL 17 database |
| `redis` | 6379 | Redis 7 for cache + pub/sub |
| `migrate` | -- | Runs goose migrations, then exits |
| `service` | 9090 | CCS gRPC server |

The service is ready when `docker compose up --wait` returns. No JWT configuration needed -- metadata auth is the default.

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

## Building the Docker Image

The repository includes a multi-stage Dockerfile at `build/Dockerfile`:

```bash
docker build -f build/Dockerfile -t central-config-service:latest .
```

The resulting image contains only the compiled binary -- no Go toolchain or source code.

## Database Migrations

CCS uses [goose](https://github.com/pressly/goose) for database migrations. Migrations live in `db/migrations/`.

### Running Migrations Manually

```bash
# Using the tools Docker image
docker build -f build/Dockerfile.tools -t ccs-tools:latest .
docker run --rm ccs-tools:latest \
  goose -dir /migrations postgres \
  "postgres://user:pass@host:5432/centralconfig?sslmode=disable" up
```

### Migration in Docker Compose

The `migrate` service in `docker-compose.yml` runs migrations automatically before the service starts. It waits for PostgreSQL to be healthy, runs `goose up`, and exits.

## Kubernetes

### Environment Variable Configuration

Configure CCS via environment variables in your Kubernetes manifests:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: central-config-service
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: ccs
          image: central-config-service:latest
          ports:
            - containerPort: 9090
              protocol: TCP
          env:
            - name: GRPC_PORT
              value: "9090"
            - name: DB_WRITE_URL
              valueFrom:
                secretKeyRef:
                  name: ccs-db
                  key: write-url
            - name: DB_READ_URL
              valueFrom:
                secretKeyRef:
                  name: ccs-db
                  key: read-url
            - name: REDIS_URL
              valueFrom:
                secretKeyRef:
                  name: ccs-redis
                  key: url
            - name: JWT_JWKS_URL
              value: "https://auth.example.com/.well-known/jwks.json"
            - name: LOG_LEVEL
              value: "info"
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

CCS exposes the standard gRPC health checking protocol, so Kubernetes gRPC probes work out of the box.

### Split Deployments

Use `ENABLE_SERVICES` to run specialized instances:

```yaml
# High-traffic config reads
- name: ENABLE_SERVICES
  value: "config"

# Admin operations (lower traffic)
- name: ENABLE_SERVICES
  value: "schema,audit"
```

All instances must connect to the same PostgreSQL database and Redis instance.

### Helm Chart

A Helm chart is planned under `deploy/helm/`. For now, use the Kubernetes manifests above as a starting point.

## Health Checks

CCS registers each enabled service with the gRPC health checking protocol. Services report `SERVING` once fully initialized:

- `centralconfig.v1.SchemaService`
- `centralconfig.v1.ConfigService`
- `centralconfig.v1.AuditService`

Use `grpc-health-probe` or Kubernetes native gRPC probes to check readiness.

## Related

- [Server Configuration](configuration.md) -- all environment variables
- [Observability](observability.md) -- OTel setup with Docker Compose
- [Getting Started](../getting-started.md) -- quick start walkthrough
