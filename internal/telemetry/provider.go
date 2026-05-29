package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

const serviceName = "decree"

// Init initializes OpenTelemetry providers and returns a shutdown function.
// The shutdown function flushes pending telemetry and releases resources.
// Standard OTel env vars (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_SERVICE_NAME, etc.)
// are respected by the SDK automatically.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	// Build the custom resource schemaless: it carries attributes but no schema
	// URL, so resource.Merge never hits a schema-URL conflict with
	// resource.Default(). Default()'s schema URL tracks the SDK's bundled
	// semconv version and changes on every otel bump (e.g. 1.40 -> 1.41), so
	// pinning a matching semconv.SchemaURL here is fragile. Merge adopts
	// Default's schema URL for the result.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	var shutdowns []func(context.Context) error

	// Trace provider.
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}
	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	shutdowns = append(shutdowns, tp.Shutdown)

	// Meter provider.
	if cfg.AnyMetrics() {
		metricExporter, metricErr := otlpmetricgrpc.New(ctx)
		if metricErr != nil {
			return nil, fmt.Errorf("create metric exporter: %w", metricErr)
		}
		mp := metric.NewMeterProvider(
			metric.WithReader(metric.NewPeriodicReader(metricExporter)),
			metric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		shutdowns = append(shutdowns, mp.Shutdown)
	}

	shutdown = func(ctx context.Context) error {
		var firstErr error
		for _, fn := range shutdowns {
			if err := fn(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	return shutdown, nil
}
