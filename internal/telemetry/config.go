package telemetry

import (
	"os"
	"strings"
)

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
	// MetricsRateLimit enables the rate-limit rejection counter (role + method attributes).
	MetricsRateLimit bool
	// MetricsValidation enables validation counters (e.g. JSON-Schema compile timeouts).
	MetricsValidation bool
	// MetricsPubSub enables the pubsub dropped-event counter.
	MetricsPubSub bool
	// MetricsAuth enables the JWKS refresh-failure counter.
	MetricsAuth bool
	// MetricsTenantAllowlist is the opt-in set of tenant IDs that receive a tenant_id label
	// on config metrics. Empty (the default) suppresses the label on all tenants to prevent
	// Prometheus/OTel cardinality explosion in deployments with many tenants.
	MetricsTenantAllowlist []string
}

// AnyMetrics returns true if any metric flag is enabled.
func (c Config) AnyMetrics() bool {
	return c.MetricsGRPC || c.MetricsDBPool || c.MetricsCache || c.MetricsConfig || c.MetricsSchema || c.MetricsRateLimit || c.MetricsValidation || c.MetricsPubSub || c.MetricsAuth
}

// ConfigFromEnv parses telemetry configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Enabled:                envBool("OTEL_ENABLED"),
		TracesGRPC:             envBool("OTEL_TRACES_GRPC"),
		TracesDB:               envBool("OTEL_TRACES_DB"),
		TracesRedis:            envBool("OTEL_TRACES_REDIS"),
		MetricsGRPC:            envBool("OTEL_METRICS_GRPC"),
		MetricsDBPool:          envBool("OTEL_METRICS_DB_POOL"),
		MetricsCache:           envBool("OTEL_METRICS_CACHE"),
		MetricsConfig:          envBool("OTEL_METRICS_CONFIG"),
		MetricsSchema:          envBool("OTEL_METRICS_SCHEMA"),
		MetricsRateLimit:       envBool("OTEL_METRICS_RATE_LIMIT"),
		MetricsValidation:      envBool("OTEL_METRICS_VALIDATION"),
		MetricsPubSub:          envBool("OTEL_METRICS_PUBSUB"),
		MetricsAuth:            envBool("OTEL_METRICS_AUTH"),
		MetricsTenantAllowlist: envStringSlice("OTEL_METRICS_TENANT_ALLOWLIST"),
	}
}

func envBool(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1"
}

func envStringSlice(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
