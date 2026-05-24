package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage"
)

func TestNewDB_MalformedDSN(t *testing.T) {
	_, err := storage.NewDB(context.Background(), "not-a-dsn", "")
	require.Error(t, err)
}

func TestWithPoolConfig_AppliesNonZeroFields(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://localhost/test")
	require.NoError(t, err)

	pc := storage.PoolConfig{
		MaxConns:          10,
		MinConns:          3,
		MaxConnLifetime:   20 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 2 * time.Minute,
	}
	storage.WithPoolConfig(pc)(cfg)

	assert.Equal(t, int32(10), cfg.MaxConns)
	assert.Equal(t, int32(3), cfg.MinConns)
	assert.Equal(t, 20*time.Minute, cfg.MaxConnLifetime)
	assert.Equal(t, 5*time.Minute, cfg.MaxConnIdleTime)
	assert.Equal(t, 2*time.Minute, cfg.HealthCheckPeriod)
}

func TestWithPoolConfig_ZeroValuesSkipped(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://localhost/test")
	require.NoError(t, err)
	cfg.MaxConns = 99
	cfg.MinConns = 7
	cfg.MaxConnLifetime = 60 * time.Minute
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = 5 * time.Minute

	storage.WithPoolConfig(storage.PoolConfig{})(cfg)

	assert.Equal(t, int32(99), cfg.MaxConns)
	assert.Equal(t, int32(7), cfg.MinConns)
	assert.Equal(t, 60*time.Minute, cfg.MaxConnLifetime)
	assert.Equal(t, 30*time.Minute, cfg.MaxConnIdleTime)
	assert.Equal(t, 5*time.Minute, cfg.HealthCheckPeriod)
}
