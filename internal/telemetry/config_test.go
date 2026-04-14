package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigFromEnv_Defaults(t *testing.T) {
	cfg := ConfigFromEnv()
	assert.False(t, cfg.Enabled)
	assert.False(t, cfg.TracesGRPC)
	assert.False(t, cfg.TracesDB)
	assert.False(t, cfg.TracesRedis)
	assert.False(t, cfg.MetricsGRPC)
	assert.False(t, cfg.MetricsDBPool)
	assert.False(t, cfg.MetricsCache)
	assert.False(t, cfg.MetricsConfig)
	assert.False(t, cfg.MetricsSchema)
}

func TestConfigFromEnv_AllEnabled(t *testing.T) {
	for _, key := range []string{
		"OTEL_ENABLED", "OTEL_TRACES_GRPC", "OTEL_TRACES_DB", "OTEL_TRACES_REDIS",
		"OTEL_METRICS_GRPC", "OTEL_METRICS_DB_POOL", "OTEL_METRICS_CACHE",
		"OTEL_METRICS_CONFIG", "OTEL_METRICS_SCHEMA",
	} {
		t.Setenv(key, "true")
	}

	cfg := ConfigFromEnv()
	assert.True(t, cfg.Enabled)
	assert.True(t, cfg.TracesGRPC)
	assert.True(t, cfg.TracesDB)
	assert.True(t, cfg.TracesRedis)
	assert.True(t, cfg.MetricsGRPC)
	assert.True(t, cfg.MetricsDBPool)
	assert.True(t, cfg.MetricsCache)
	assert.True(t, cfg.MetricsConfig)
	assert.True(t, cfg.MetricsSchema)
}

func TestConfigFromEnv_NumericOne(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "1")
	cfg := ConfigFromEnv()
	assert.True(t, cfg.Enabled)
}

func TestConfigFromEnv_InvalidValue(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "yes")
	cfg := ConfigFromEnv()
	assert.False(t, cfg.Enabled)
}

func TestAnyMetrics_NoneEnabled(t *testing.T) {
	cfg := Config{}
	assert.False(t, cfg.AnyMetrics())
}

func TestAnyMetrics_OneEnabled(t *testing.T) {
	assert.True(t, Config{MetricsCache: true}.AnyMetrics())
	assert.True(t, Config{MetricsGRPC: true}.AnyMetrics())
	assert.True(t, Config{MetricsDBPool: true}.AnyMetrics())
	assert.True(t, Config{MetricsConfig: true}.AnyMetrics())
	assert.True(t, Config{MetricsSchema: true}.AnyMetrics())
}
