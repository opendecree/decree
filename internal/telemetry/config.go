package telemetry

import "os"

// Config holds the telemetry feature flags parsed from environment variables.
type Config struct {
	// Enabled is the master switch — initializes SDK, OTLP exporter, and slog trace correlation.
	Enabled bool
	// TracesGRPC enables gRPC server spans (per-RPC method, status, duration).
	TracesGRPC bool
	// TracesDB enables pgx query/transaction spans.
	TracesDB bool
	// TracesRedis enables Redis command spans.
	TracesRedis bool
	// MetricsGRPC enables built-in otelgrpc request count, latency, and message size metrics.
	MetricsGRPC bool
	// MetricsDBPool enables DB connection pool gauges (total, acquired, idle connections).
	MetricsDBPool bool
	// MetricsCache enables cache hit/miss counters.
	MetricsCache bool
	// MetricsConfig enables config write counters and version gauge per tenant.
	MetricsConfig bool
	// MetricsSchema enables schema publish counter.
	MetricsSchema bool
}

// AnyMetrics returns true if any metric flag is enabled.
func (c Config) AnyMetrics() bool {
	return c.MetricsGRPC || c.MetricsDBPool || c.MetricsCache || c.MetricsConfig || c.MetricsSchema
}

// ConfigFromEnv parses telemetry configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Enabled:       envBool("OTEL_ENABLED"),
		TracesGRPC:    envBool("OTEL_TRACES_GRPC"),
		TracesDB:      envBool("OTEL_TRACES_DB"),
		TracesRedis:   envBool("OTEL_TRACES_REDIS"),
		MetricsGRPC:   envBool("OTEL_METRICS_GRPC"),
		MetricsDBPool: envBool("OTEL_METRICS_DB_POOL"),
		MetricsCache:  envBool("OTEL_METRICS_CACHE"),
		MetricsConfig: envBool("OTEL_METRICS_CONFIG"),
		MetricsSchema: envBool("OTEL_METRICS_SCHEMA"),
	}
}

func envBool(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1"
}
