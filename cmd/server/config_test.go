package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opendecree/decree/internal/schema"
	"github.com/opendecree/decree/internal/storage"
	"github.com/opendecree/decree/internal/validation"
)

func TestParseEnvDuration_Unset(t *testing.T) {
	t.Setenv("_TEST_DUR_UNSET", "")
	assert.Equal(t, 5*time.Second, parseEnvDuration("_TEST_DUR_UNSET", 5*time.Second))
}

func TestParseEnvDuration_Valid(t *testing.T) {
	t.Setenv("_TEST_DUR_VALID", "2m30s")
	assert.Equal(t, 2*time.Minute+30*time.Second, parseEnvDuration("_TEST_DUR_VALID", 0))
}

func TestLoadConfig_DBPoolEnvVars(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "50")
	t.Setenv("DB_MIN_CONNS", "5")
	t.Setenv("DB_MAX_CONN_LIFETIME", "1h")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "15m")
	t.Setenv("DB_HEALTH_CHECK_PERIOD", "2m")

	cfg := loadConfig()

	assert.Equal(t, 50, cfg.DBMaxConns)
	assert.Equal(t, 5, cfg.DBMinConns)
	assert.Equal(t, time.Hour, cfg.DBMaxConnLifetime)
	assert.Equal(t, 15*time.Minute, cfg.DBMaxConnIdleTime)
	assert.Equal(t, 2*time.Minute, cfg.DBHealthCheckPeriod)
}

// TestDefaultLimitsRegexCapActive asserts that the regex-length ReDoS cap is
// non-zero in the default limits for both the schema and validation packages.
// A zero value disables the cap entirely, which is the bug fixed in #676.
func TestDefaultLimitsRegexCapActive(t *testing.T) {
	sl := schema.DefaultLimits()
	assert.Greater(t, sl.RegexPatternMaxLength, 0,
		"schema.DefaultLimits().RegexPatternMaxLength must be > 0 to guard against ReDoS")

	vl := validation.DefaultLimits()
	assert.Greater(t, vl.RegexMaxLength, 0,
		"validation.DefaultLimits().RegexMaxLength must be > 0 to guard against ReDoS")
}

func TestPoolConfigFromServerCfg(t *testing.T) {
	cfg := serverConfig{
		DBMaxConns:          10,
		DBMinConns:          3,
		DBMaxConnLifetime:   20 * time.Minute,
		DBMaxConnIdleTime:   5 * time.Minute,
		DBHealthCheckPeriod: 2 * time.Minute,
	}
	got := poolConfigFromServerCfg(cfg)
	assert.Equal(t, storage.PoolConfig{
		MaxConns:          10,
		MinConns:          3,
		MaxConnLifetime:   20 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 2 * time.Minute,
	}, got)
}
