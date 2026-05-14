package ratelimit_test

import (
	"bytes"
	"context"
	"log/slog"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/ratelimit"
)

// countingCounter wraps noop.Int64Counter to track Add calls.
type countingCounter struct {
	noop.Int64Counter
	n atomic.Int64
}

func (c *countingCounter) Add(_ context.Context, incr int64, _ ...metric.AddOption) {
	c.n.Add(incr)
}

func TestWithInterceptorLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	denying := ratelimit.NewInProcess(rate.Limit(0), 0)
	i := ratelimit.New(ratelimit.Config{Anonymous: denying},
		ratelimit.WithInterceptorLogger(logger),
	)

	// Health-check methods emit a debug log via the configured logger.
	err := invokeUnary(t, i, context.Background(), "/grpc.health.v1.Health/Check")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "rate limit exempt")
}

func TestWithRejectedCounter(t *testing.T) {
	counter := &countingCounter{}

	denying := ratelimit.NewInProcess(rate.Limit(0), 0)
	i := ratelimit.New(ratelimit.Config{Anonymous: denying},
		ratelimit.WithRejectedCounter(counter),
	)

	err := invokeUnary(t, i, context.Background(), testMethod)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Equal(t, int64(1), counter.n.Load())
}
