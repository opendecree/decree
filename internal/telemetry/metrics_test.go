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
	// Should return immediately without panic.
	StartDBPoolMetrics(context.Background(), Config{}, nil, nil)
	StartDBPoolMetrics(context.Background(), Config{Enabled: true, MetricsDBPool: false}, nil, nil)
}
