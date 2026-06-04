# Grafana Dashboard — OpenDecree Service Overview

Pre-built Grafana 10+ dashboard for monitoring the decree server via OpenTelemetry metrics.

## Importing the Dashboard

1. Open Grafana → **Dashboards** → **Import**.
2. Click **Upload dashboard JSON file** and select `dashboard.json`.
3. When prompted, select the **Prometheus** datasource that scrapes the decree OTel exporter.
4. Click **Import**.

## Required Datasource

The dashboard uses a Prometheus datasource. The decree server exposes metrics via an OTel Prometheus exporter (default port `9090`, path `/metrics`). Configure Prometheus to scrape that endpoint, then add it as a datasource in Grafana.

Example Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: decree
    static_configs:
      - targets: ["decree-service:9090"]
```

## Metric Namespace and OTel-to-Prometheus Translation

The decree server registers metrics under the `decree` OTel meter. The OTel Prometheus exporter applies these transformations before exposing metrics:

- Dots (`.`) in metric names become underscores (`_`).
- The meter name (`decree`) is prepended as a namespace: `decree_<metric>`.
- Counters (OTel `Int64Counter`) get a `_total` suffix.

Examples:

| OTel name | Prometheus name |
|-----------|-----------------|
| `db.pool.acquired_connections` | `decree_db_pool_acquired_connections` |
| `config.cache.hits` | `decree_config_cache_hits_total` |
| `config.writes` | `decree_config_writes_total` |
| `ratelimit.rejected` | `decree_ratelimit_rejected_total` |
| `pubsub.dropped_total` | `decree_pubsub_dropped_total` |
| `auth.jwks_refresh_failures_total` | `decree_auth_jwks_refresh_failures_total` |
| `validation.json_schema_compile_timeouts_total` | `decree_validation_json_schema_compile_timeouts_total` |
| `validation.json_schema_compiles_in_flight` | `decree_validation_json_schema_compiles_in_flight` |

Standard gRPC metrics emitted by `otelgrpc` are not namespaced and use `rpc_` prefix (e.g., `rpc_server_duration_milliseconds_bucket`).

## Dashboard Panels

| Row | Panel | Type | What it shows |
|-----|-------|------|---------------|
| Database Connection Pool | DB Pool Utilization (write) | Gauge | `acquired / max` for write pool; yellow ≥ 80%, red ≥ 95% |
| Database Connection Pool | DB Pool Utilization (read) | Gauge | Same for read pool |
| Database Connection Pool | DB Pool Connections | Time series | Acquired, idle, total for both pools |
| Config Operations | Config Write Rate | Stat | Writes/min (5-minute rate) |
| Config Operations | Cache Hit Rate | Time series | `hits / (hits + misses)` |
| Reliability | Rate Limit Rejections | Stat | Rejections/min (5-minute rate) |
| Reliability | PubSub Drops | Stat | Drops/min (5-minute rate) |
| Reliability | JWKS Refresh Failures | Stat | Failures in the last hour; red threshold at > 0 |
| gRPC Latency | gRPC P99 Latency | Time series | P99 server duration by method |
