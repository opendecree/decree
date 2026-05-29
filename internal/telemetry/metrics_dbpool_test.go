package telemetry

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestStartDBPoolMetrics_DisabledIsNoOp(t *testing.T) {
	// Master switch off, then metric flag off — both return before touching the meter.
	StartDBPoolMetrics(context.Background(), Config{Enabled: false}, nil, nil)
	StartDBPoolMetrics(context.Background(), Config{Enabled: true, MetricsDBPool: false}, nil, nil)
}

func TestStartDBPoolMetrics_RegistersGauges(t *testing.T) {
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(noop.NewMeterProvider())
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	cfg := Config{Enabled: true, MetricsDBPool: true}

	// Identical write/read pool → single-callback branch. The noop meter never
	// invokes the observe callback, so the nil pool is never dereferenced.
	StartDBPoolMetrics(context.Background(), cfg, nil, nil)

	// Distinct pools → two-callback branch.
	writePool, readPool := &pgxpool.Pool{}, &pgxpool.Pool{}
	StartDBPoolMetrics(context.Background(), cfg, writePool, readPool)
}
