package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCacheMetrics_Disabled(t *testing.T) {
	assert.Nil(t, NewCacheMetrics(Config{}))
	assert.Nil(t, NewCacheMetrics(Config{Enabled: true, MetricsCache: false}))
}

func TestNewCacheMetrics_Enabled(t *testing.T) {
	m := NewCacheMetrics(Config{Enabled: true, MetricsCache: true})
	assert.NotNil(t, m)
}

func TestCacheMetrics_NilSafe(t *testing.T) {
	var m *CacheMetrics
	m.Hit(context.Background())
	m.Miss(context.Background())
}

func TestCacheMetrics_Hit_Miss(t *testing.T) {
	m := NewCacheMetrics(Config{Enabled: true, MetricsCache: true})
	m.Hit(context.Background())
	m.Miss(context.Background())
}

func TestNewConfigMetrics_Disabled(t *testing.T) {
	assert.Nil(t, NewConfigMetrics(Config{}))
	assert.Nil(t, NewConfigMetrics(Config{Enabled: true, MetricsConfig: false}))
}

func TestNewConfigMetrics_Enabled(t *testing.T) {
	m := NewConfigMetrics(Config{Enabled: true, MetricsConfig: true})
	assert.NotNil(t, m)
}

func TestConfigMetrics_NilSafe(t *testing.T) {
	var m *ConfigMetrics
	m.RecordWrite(context.Background(), "t1", "set_field")
	m.RecordVersion(context.Background(), "t1", 5)
}

func TestConfigMetrics_RecordWrite_RecordVersion(t *testing.T) {
	m := NewConfigMetrics(Config{Enabled: true, MetricsConfig: true})
	m.RecordWrite(context.Background(), "t1", "set_field")
	m.RecordVersion(context.Background(), "t1", 5)
}

func TestConfigMetrics_AllowlistEmptyByDefault(t *testing.T) {
	// No allowlist — tenant_id label is suppressed; recording must not panic.
	m := NewConfigMetrics(Config{Enabled: true, MetricsConfig: true})
	m.RecordWrite(context.Background(), "t1", "set_field")
	m.RecordVersion(context.Background(), "t1", 5)
}

func TestConfigMetrics_AllowlistLabelsMatchingTenants(t *testing.T) {
	// Only tenants in the allowlist receive the label; others must still record without panic.
	m := NewConfigMetrics(Config{
		Enabled:                true,
		MetricsConfig:          true,
		MetricsTenantAllowlist: []string{"tenant-a"},
	})
	m.RecordWrite(context.Background(), "tenant-a", "set_field") // in allowlist
	m.RecordWrite(context.Background(), "tenant-b", "set_field") // not in allowlist
	m.RecordVersion(context.Background(), "tenant-a", 3)
	m.RecordVersion(context.Background(), "tenant-b", 7)
}

func TestNewSchemaMetrics_Disabled(t *testing.T) {
	assert.Nil(t, NewSchemaMetrics(Config{}))
	assert.Nil(t, NewSchemaMetrics(Config{Enabled: true, MetricsSchema: false}))
}

func TestNewSchemaMetrics_Enabled(t *testing.T) {
	m := NewSchemaMetrics(Config{Enabled: true, MetricsSchema: true})
	assert.NotNil(t, m)
}

func TestSchemaMetrics_NilSafe(t *testing.T) {
	var m *SchemaMetrics
	m.RecordPublish(context.Background())
}

func TestSchemaMetrics_RecordPublish(t *testing.T) {
	m := NewSchemaMetrics(Config{Enabled: true, MetricsSchema: true})
	m.RecordPublish(context.Background())
}

func TestStartDBPoolMetrics_Disabled(t *testing.T) {
	StartDBPoolMetrics(context.Background(), Config{}, nil, nil)
	StartDBPoolMetrics(context.Background(), Config{Enabled: true, MetricsDBPool: false}, nil, nil)
}

func TestStartDBPoolMetrics_Enabled(t *testing.T) {
	// nil pools: the registered callback is never invoked by the no-op global meter,
	// so no nil dereference occurs. Exercises gauge registration and callback wiring.
	StartDBPoolMetrics(context.Background(), Config{Enabled: true, MetricsDBPool: true}, nil, nil)
}

func TestNewRateLimitMetrics_Disabled(t *testing.T) {
	assert.Nil(t, NewRateLimitMetrics(Config{}))
	assert.Nil(t, NewRateLimitMetrics(Config{Enabled: true, MetricsRateLimit: false}))
}

func TestNewRateLimitMetrics_Enabled(t *testing.T) {
	m := NewRateLimitMetrics(Config{Enabled: true, MetricsRateLimit: true})
	assert.NotNil(t, m)
}

func TestRateLimitMetrics_NilSafe(t *testing.T) {
	var m *RateLimitMetrics
	counter, ok := m.Counter()
	assert.False(t, ok)
	assert.Nil(t, counter)
}

func TestRateLimitMetrics_Counter(t *testing.T) {
	m := NewRateLimitMetrics(Config{Enabled: true, MetricsRateLimit: true})
	counter, ok := m.Counter()
	assert.True(t, ok)
	assert.NotNil(t, counter)
}

func TestNewValidationMetrics_Disabled(t *testing.T) {
	assert.Nil(t, NewValidationMetrics(Config{}))
	assert.Nil(t, NewValidationMetrics(Config{Enabled: true, MetricsValidation: false}))
}

func TestNewValidationMetrics_Enabled(t *testing.T) {
	m := NewValidationMetrics(Config{Enabled: true, MetricsValidation: true})
	assert.NotNil(t, m)
}

func TestValidationMetrics_NilSafe(t *testing.T) {
	var m *ValidationMetrics
	counter, ok := m.TimeoutCounter()
	assert.False(t, ok)
	assert.Nil(t, counter)
	counter, ok = m.RegexErrorCounter()
	assert.False(t, ok)
	assert.Nil(t, counter)
	gauge, ok := m.InFlightGauge()
	assert.False(t, ok)
	assert.Nil(t, gauge)
	counter, ok = m.CelCapExceededCounter()
	assert.False(t, ok)
	assert.Nil(t, counter)
}

func TestValidationMetrics_CelCapExceededCounter(t *testing.T) {
	m := NewValidationMetrics(Config{Enabled: true, MetricsValidation: true})
	counter, ok := m.CelCapExceededCounter()
	assert.True(t, ok)
	assert.NotNil(t, counter)
}

func TestValidationMetrics_InFlightGauge(t *testing.T) {
	m := NewValidationMetrics(Config{Enabled: true, MetricsValidation: true})
	gauge, ok := m.InFlightGauge()
	assert.True(t, ok)
	assert.NotNil(t, gauge)
}

func TestValidationMetrics_TimeoutCounter(t *testing.T) {
	m := NewValidationMetrics(Config{Enabled: true, MetricsValidation: true})
	counter, ok := m.TimeoutCounter()
	assert.True(t, ok)
	assert.NotNil(t, counter)
}

func TestValidationMetrics_RegexErrorCounter(t *testing.T) {
	m := NewValidationMetrics(Config{Enabled: true, MetricsValidation: true})
	counter, ok := m.RegexErrorCounter()
	assert.True(t, ok)
	assert.NotNil(t, counter)
}
