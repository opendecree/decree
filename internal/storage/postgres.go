package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB holds separate connection pools for read and write operations.
type DB struct {
	WritePool *pgxpool.Pool
	ReadPool  *pgxpool.Pool
}

// Option configures the database connection pools.
type Option func(*pgxpool.Config)

// PoolConfig holds connection pool sizing parameters.
type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// WithTracer adds a pgx query tracer to the connection pool.
func WithTracer(tracer pgx.QueryTracer) Option {
	return func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = tracer
	}
}

// WithPoolConfig overrides connection pool sizing defaults.
func WithPoolConfig(pc PoolConfig) Option {
	return func(cfg *pgxpool.Config) {
		if pc.MaxConns > 0 {
			cfg.MaxConns = pc.MaxConns
		}
		if pc.MinConns > 0 {
			cfg.MinConns = pc.MinConns
		}
		if pc.MaxConnLifetime > 0 {
			cfg.MaxConnLifetime = pc.MaxConnLifetime
		}
		if pc.MaxConnIdleTime > 0 {
			cfg.MaxConnIdleTime = pc.MaxConnIdleTime
		}
		if pc.HealthCheckPeriod > 0 {
			cfg.HealthCheckPeriod = pc.HealthCheckPeriod
		}
	}
}

// NewDB creates connection pools for the given DSNs.
// If readDSN is empty, the write pool is used for reads.
func NewDB(ctx context.Context, writeDSN, readDSN string, opts ...Option) (*DB, error) {
	writePool, err := newPool(ctx, writeDSN, opts)
	if err != nil {
		return nil, fmt.Errorf("write db: %w", err)
	}

	readPool := writePool
	if readDSN != "" && readDSN != writeDSN {
		readPool, err = newPool(ctx, readDSN, opts)
		if err != nil {
			writePool.Close()
			return nil, fmt.Errorf("read db: %w", err)
		}
	}

	return &DB{
		WritePool: writePool,
		ReadPool:  readPool,
	}, nil
}

func newPool(ctx context.Context, dsn string, opts []Option) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.MaxConns = 25
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 10 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute
	for _, opt := range opts {
		opt(cfg)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Close closes both connection pools.
func (db *DB) Close() {
	if db.ReadPool != db.WritePool {
		db.ReadPool.Close()
	}
	db.WritePool.Close()
}
