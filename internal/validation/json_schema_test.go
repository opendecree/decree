package validation

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingCounter wraps noop.Int64Counter to track Add calls.
type countingCounter struct {
	noop.Int64Counter
	n atomic.Int64
}

func (c *countingCounter) Add(_ context.Context, incr int64, _ ...metric.AddOption) {
	c.n.Add(incr)
}

// countingGauge wraps noop.Int64UpDownCounter to track the running sum of Add calls.
type countingGauge struct {
	noop.Int64UpDownCounter
	n atomic.Int64
}

func (g *countingGauge) Add(_ context.Context, incr int64, _ ...metric.AddOption) {
	g.n.Add(incr)
}

func optsWithLimits(l Limits) options {
	return resolveOptions([]Option{WithLimits(l)})
}

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	assert.Equal(t, 5*time.Second, l.CompileTimeout)
	assert.Equal(t, 64, l.MaxDepth)
	assert.Equal(t, 32, l.MaxConcurrentCompiles)
}

func TestNewJSONSchemaValidator_Compiles(t *testing.T) {
	doc := `{"type":"object","properties":{"name":{"type":"string"}}}`
	v, err := newJSONSchemaValidator(doc, optsWithLimits(DefaultLimits()))
	require.NoError(t, err)
	require.NotNil(t, v)
	require.NoError(t, v.validate(`{"name":"x"}`))
	require.Error(t, v.validate(`{"name":1}`))
}

func TestNewJSONSchemaValidator_DepthExceeded(t *testing.T) {
	// Build a schema with nesting depth 10, then cap MaxDepth to 5.
	doc := strings.Repeat(`{"properties":{"x":`, 10) + `{"type":"string"}` + strings.Repeat(`}}`, 10)
	_, err := newJSONSchemaValidator(doc, optsWithLimits(Limits{MaxDepth: 5}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nesting depth exceeds limit of 5")
}

func TestNewJSONSchemaValidator_DepthDisabled(t *testing.T) {
	doc := strings.Repeat(`{"properties":{"x":`, 5) + `{"type":"string"}` + strings.Repeat(`}}`, 5)
	v, err := newJSONSchemaValidator(doc, optsWithLimits(Limits{MaxDepth: 0}))
	require.NoError(t, err)
	require.NotNil(t, v)
}

func TestNewJSONSchemaValidator_MalformedJSONFallsThrough(t *testing.T) {
	// Pre-scan ignores non-JSON; compiler reports the syntax error.
	_, err := newJSONSchemaValidator(`not-json`, optsWithLimits(DefaultLimits()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json schema")
}

func TestNewJSONSchemaValidator_TimeoutZeroIsUnbounded(t *testing.T) {
	doc := `{"type":"string"}`
	v, err := newJSONSchemaValidator(doc, optsWithLimits(Limits{CompileTimeout: 0, MaxDepth: 0}))
	require.NoError(t, err)
	require.NotNil(t, v)
}

func TestNewJSONSchemaValidator_CompileTimeoutFires(t *testing.T) {
	// A 1ns timeout has timer.C ready before the compile goroutine can be
	// scheduled, unmarshal the document, and push to the (buffered) result
	// channel — the select therefore reaches the timeout branch.
	doc := `{"type":"object","properties":{"name":{"type":"string"}}}`
	v, err := newJSONSchemaValidator(doc, optsWithLimits(Limits{CompileTimeout: 1 * time.Nanosecond, MaxDepth: 0}))
	require.Error(t, err)
	require.Nil(t, v)
	assert.Contains(t, err.Error(), "compile json schema: timeout after")
}

func TestNewJSONSchemaValidator_TimeoutIncrementsCounter(t *testing.T) {
	// Verify that the timeout branch increments the provided OTEL counter.
	counter := &countingCounter{}
	doc := `{"type":"object","properties":{"name":{"type":"string"}}}`
	opts := resolveOptions([]Option{
		WithLimits(Limits{CompileTimeout: 1 * time.Nanosecond, MaxDepth: 0}),
		WithTimeoutCounter(counter),
	})
	_, err := newJSONSchemaValidator(doc, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compile json schema: timeout after")
	assert.Equal(t, int64(1), counter.n.Load(), "timeout counter should be incremented exactly once")
}

func TestNewJSONSchemaValidator_SuccessDoesNotIncrementCounter(t *testing.T) {
	// Successful compile must not increment the counter.
	counter := &countingCounter{}
	doc := `{"type":"string"}`
	opts := resolveOptions([]Option{
		WithLimits(DefaultLimits()),
		WithTimeoutCounter(counter),
	})
	v, err := newJSONSchemaValidator(doc, opts)
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, int64(0), counter.n.Load(), "counter must not be incremented on success")
}

func TestNewJSONSchemaValidator_SemaphoreFullTimeoutsOnWait(t *testing.T) {
	// With a full semaphore (MaxConcurrentCompiles=1, slot pre-consumed) and a
	// short timeout, newJSONSchemaValidator should return a timeout error while
	// waiting for a slot — not a compile error.
	opts := resolveOptions([]Option{
		WithLimits(Limits{
			MaxConcurrentCompiles: 1,
			CompileTimeout:        10 * time.Millisecond,
		}),
	})
	// Pre-fill the semaphore to simulate a running compile occupying the slot.
	opts.compileSem <- struct{}{}

	doc := `{"type":"string"}`
	_, err := newJSONSchemaValidator(doc, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout after")
}

func TestNewJSONSchemaValidator_InFlightGaugeSuccessPath(t *testing.T) {
	// Gauge must return to 0 after a successful compile.
	// The gauge is decremented before the goroutine sends its result, so the
	// value is guaranteed 0 by the time newJSONSchemaValidator returns.
	gauge := &countingGauge{}
	doc := `{"type":"string"}`
	opts := resolveOptions([]Option{
		WithLimits(DefaultLimits()),
		WithInFlightGauge(gauge),
	})
	v, err := newJSONSchemaValidator(doc, opts)
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, int64(0), gauge.n.Load(), "gauge must return to 0 after successful compile")
}

func TestNewJSONSchemaValidator_InFlightGaugeMalformedJSON(t *testing.T) {
	// Gauge must return to 0 when the compile goroutine exits early with an error.
	gauge := &countingGauge{}
	opts := resolveOptions([]Option{
		WithLimits(DefaultLimits()),
		WithInFlightGauge(gauge),
	})
	_, err := newJSONSchemaValidator(`not-json`, opts)
	require.Error(t, err)
	assert.Equal(t, int64(0), gauge.n.Load(), "gauge must return to 0 after compile error")
}

func TestScanJSONDepth(t *testing.T) {
	// Object nesting.
	require.NoError(t, scanJSONDepth(`{"a":{"b":{"c":1}}}`, 5))
	require.Error(t, scanJSONDepth(`{"a":{"b":{"c":1}}}`, 2))

	// Array nesting counts too.
	require.NoError(t, scanJSONDepth(`[[[[1]]]]`, 5))
	require.Error(t, scanJSONDepth(`[[[[1]]]]`, 3))

	// Non-JSON: scan is a no-op.
	require.NoError(t, scanJSONDepth(`not json`, 0))
}
