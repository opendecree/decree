package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

// saveOtelGlobals snapshots the global tracer/meter providers so a test that
// calls Init (which sets them) can restore them afterward and not leak a
// shut-down provider into sibling tests.
func saveOtelGlobals(t *testing.T) {
	t.Helper()
	tp := otel.GetTracerProvider()
	mp := otel.GetMeterProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetMeterProvider(mp)
	})
}

// shutdownContext returns an already-cancelled context for shutdown calls. No
// OTLP collector runs in unit tests; a cancelled context makes the exporters'
// final flush return immediately instead of retrying against a dead endpoint
// until a deadline, keeping the test fast.
func shutdownContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestInit_Disabled_ReturnsNoopShutdown(t *testing.T) {
	shutdown, err := Init(context.Background(), Config{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// No-op shutdown should succeed with a nil context and not panic.
	assert.NoError(t, shutdown(context.Background()))
}

func TestInit_Enabled_TracesOnly(t *testing.T) {
	saveOtelGlobals(t)

	// No metric flags set → only the trace provider is wired (AnyMetrics false).
	shutdown, err := Init(context.Background(), Config{Enabled: true})
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Exporters connect lazily, so Init succeeds without a collector. Shutdown
	// flushes against a dead endpoint; we only require that it returns.
	_ = shutdown(shutdownContext(t))
}

func TestInit_Enabled_WithMetrics(t *testing.T) {
	saveOtelGlobals(t)

	// A metric flag set → both trace and meter providers are wired.
	shutdown, err := Init(context.Background(), Config{Enabled: true, MetricsGRPC: true})
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	_ = shutdown(shutdownContext(t))
}
