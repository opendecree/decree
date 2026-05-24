package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
