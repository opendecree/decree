# Prometheus Alerting Rules

Alerting rules for the Decree service, covering DB pool exhaustion, cache health,
pub/sub reliability, rate limiting, JWKS refresh, and CEL validation.

## Loading the rules

Add a `rule_files` entry to your `prometheus.yml`:

```yaml
rule_files:
  - /path/to/deploy/prometheus/alerts.yaml
```

Prometheus reloads rule files on `SIGHUP` or via the `/-/reload` HTTP endpoint
(requires `--web.enable-lifecycle`).

## Metrics source

Metrics are emitted by the Decree server using the **OpenTelemetry Go SDK** and
exported to Prometheus via the OTel Prometheus exporter (configured in
`deploy/otel-collector.yaml`).

### OTel-to-Prometheus name translation

OTel metric names use `.` as a separator; Prometheus uses `_`. The exporter
translates automatically:

| OTel name | Prometheus name |
|-----------|----------------|
| `db.pool.acquired_connections` | `db_pool_acquired_connections` |
| `config.cache.hits` | `config_cache_hits_total` |

Counter instruments also receive a `_total` suffix per the Prometheus
exposition format convention.

## Alert groups

| Group | Alerts |
|-------|--------|
| `decree.db` | `DecreeDBPoolExhaustion` (critical), `DecreeDBPoolHighUtilization` (warning) |
| `decree.cache` | `DecreeCacheMissRateHigh` (warning) |
| `decree.reliability` | `DecreePubSubDropped`, `DecreeRateLimitRejectionHigh`, `DecreeJWKSRefreshFailing` (critical) |
| `decree.validation` | `DecreeCELCostCapExceeded` |

> **Alpha**: Decree is pre-production software. Thresholds and alert definitions
> are subject to change.
