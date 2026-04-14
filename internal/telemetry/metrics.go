package telemetry

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "central-config-service"

// CacheMetrics records cache hit/miss counters.
type CacheMetrics struct {
	hits   metric.Int64Counter
	misses metric.Int64Counter
}

// NewCacheMetrics creates cache metrics. Returns nil if not enabled.
func NewCacheMetrics(cfg Config) *CacheMetrics {
	if !cfg.Enabled || !cfg.MetricsCache {
		return nil
	}
	meter := otel.Meter(meterName)
	hits, _ := meter.Int64Counter("config.cache.hits",
		metric.WithDescription("Number of cache hits"))
	misses, _ := meter.Int64Counter("config.cache.misses",
		metric.WithDescription("Number of cache misses"))
	return &CacheMetrics{hits: hits, misses: misses}
}

// Hit records a cache hit.
func (m *CacheMetrics) Hit(ctx context.Context) {
	if m != nil {
		m.hits.Add(ctx, 1)
	}
}

// Miss records a cache miss.
func (m *CacheMetrics) Miss(ctx context.Context) {
	if m != nil {
		m.misses.Add(ctx, 1)
	}
}

// ConfigMetrics records config write counters and version gauge.
type ConfigMetrics struct {
	writes   metric.Int64Counter
	versions metric.Int64Gauge
}

// NewConfigMetrics creates config metrics. Returns nil if not enabled.
func NewConfigMetrics(cfg Config) *ConfigMetrics {
	if !cfg.Enabled || !cfg.MetricsConfig {
		return nil
	}
	meter := otel.Meter(meterName)
	writes, _ := meter.Int64Counter("config.writes",
		metric.WithDescription("Number of config write operations"))
	versions, _ := meter.Int64Gauge("config.versions",
		metric.WithDescription("Current config version number per tenant"))
	return &ConfigMetrics{writes: writes, versions: versions}
}

// RecordWrite records a config write event.
func (m *ConfigMetrics) RecordWrite(ctx context.Context, tenantID, action string) {
	if m != nil {
		m.writes.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("tenant_id", tenantID),
				attribute.String("action", action),
			))
	}
}

// RecordVersion records the current config version for a tenant.
func (m *ConfigMetrics) RecordVersion(ctx context.Context, tenantID string, version int64) {
	if m != nil {
		m.versions.Record(ctx, version,
			metric.WithAttributes(attribute.String("tenant_id", tenantID)))
	}
}

// SchemaMetrics records schema lifecycle counters.
type SchemaMetrics struct {
	publishes metric.Int64Counter
}

// NewSchemaMetrics creates schema metrics. Returns nil if not enabled.
func NewSchemaMetrics(cfg Config) *SchemaMetrics {
	if !cfg.Enabled || !cfg.MetricsSchema {
		return nil
	}
	meter := otel.Meter(meterName)
	publishes, _ := meter.Int64Counter("schema.publishes",
		metric.WithDescription("Number of schema publish events"))
	return &SchemaMetrics{publishes: publishes}
}

// RecordPublish records a schema publish event.
func (m *SchemaMetrics) RecordPublish(ctx context.Context) {
	if m != nil {
		m.publishes.Add(ctx, 1)
	}
}

// StartDBPoolMetrics starts a background goroutine that periodically records
// DB connection pool statistics. Returns immediately if not enabled.
func StartDBPoolMetrics(ctx context.Context, cfg Config, writePool, readPool *pgxpool.Pool) {
	if !cfg.Enabled || !cfg.MetricsDBPool {
		return
	}
	meter := otel.Meter(meterName)
	totalConns, _ := meter.Int64ObservableGauge("db.pool.total_connections",
		metric.WithDescription("Total number of connections in the pool"))
	acquiredConns, _ := meter.Int64ObservableGauge("db.pool.acquired_connections",
		metric.WithDescription("Number of currently acquired connections"))
	idleConns, _ := meter.Int64ObservableGauge("db.pool.idle_connections",
		metric.WithDescription("Number of idle connections in the pool"))

	record := func(pool *pgxpool.Pool, poolName string) func(context.Context, metric.Observer) error {
		return func(ctx context.Context, o metric.Observer) error {
			stat := pool.Stat()
			attrs := metric.WithAttributes(attribute.String("pool", poolName))
			o.ObserveInt64(totalConns, int64(stat.TotalConns()), attrs)
			o.ObserveInt64(acquiredConns, int64(stat.AcquiredConns()), attrs)
			o.ObserveInt64(idleConns, int64(stat.IdleConns()), attrs)
			return nil
		}
	}

	callbacks := []metric.Observable{totalConns, acquiredConns, idleConns}
	if writePool == readPool {
		_, _ = meter.RegisterCallback(record(writePool, "write"), callbacks...)
	} else {
		_, _ = meter.RegisterCallback(record(writePool, "write"), callbacks...)
		_, _ = meter.RegisterCallback(record(readPool, "read"), callbacks...)
	}

	// Keep reference alive — the callbacks are registered with the meter provider
	// and will be called on each collection interval. No goroutine needed for
	// observable gauges — the SDK calls the callback.
	_ = time.Now() // suppress unused import if needed
}
