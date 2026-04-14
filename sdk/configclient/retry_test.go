package configclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetry_NoRetryByDefault(t *testing.T) {
	calls := 0
	c := &Client{}

	result, err := retry(context.Background(), c, func(_ context.Context) (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "down")
	})

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Equal(t, 1, calls, "should not retry when retry is disabled")
}

func TestRetry_RetriesOnUnavailable(t *testing.T) {
	calls := 0
	c := &Client{opts: options{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			RetryableCheck: IsRetryable,
		},
	}}

	result, err := retry(context.Background(), c, func(_ context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "", status.Error(codes.Unavailable, "down")
		}
		return "ok", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", result)
	assert.Equal(t, 3, calls, "should have retried twice before succeeding")
}

func TestRetry_DoesNotRetryNonRetryable(t *testing.T) {
	calls := 0
	c := &Client{opts: options{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			RetryableCheck: IsRetryable,
		},
	}}

	_, err := retry(context.Background(), c, func(_ context.Context) (string, error) {
		calls++
		return "", status.Error(codes.NotFound, "not found")
	})

	assert.Error(t, err)
	assert.Equal(t, 1, calls, "should not retry NotFound")
}

func TestRetry_ExhaustsAttempts(t *testing.T) {
	calls := 0
	c := &Client{opts: options{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			RetryableCheck: IsRetryable,
		},
	}}

	_, err := retry(context.Background(), c, func(_ context.Context) (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "always down")
	})

	assert.Error(t, err)
	assert.Equal(t, 3, calls)
}

func TestRetry_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	calls := 0
	c := &Client{opts: options{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    10,
			InitialBackoff: time.Second,
			RetryableCheck: IsRetryable,
		},
	}}

	_, err := retry(ctx, c, func(_ context.Context) (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "down")
	})

	// First call executes, then context is already cancelled before backoff.
	assert.Equal(t, 1, calls)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRetryConfig_Defaults(t *testing.T) {
	cfg := RetryConfig{}.withDefaults()
	assert.Equal(t, 3, cfg.MaxAttempts)
	assert.Equal(t, 100*time.Millisecond, cfg.InitialBackoff)
	assert.Equal(t, 5*time.Second, cfg.MaxBackoff)
	assert.NotNil(t, cfg.RetryableCheck)
}

func TestRetryConfig_PreservesCustomValues(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:    5,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
	}.withDefaults()
	assert.Equal(t, 5, cfg.MaxAttempts)
	assert.Equal(t, 200*time.Millisecond, cfg.InitialBackoff)
	assert.Equal(t, 10*time.Second, cfg.MaxBackoff)
}

func TestBackoffDuration(t *testing.T) {
	// Without jitter, exponential: 100ms, 200ms, 400ms...
	b0 := backoffDuration(0, 100*time.Millisecond, 5*time.Second, false)
	assert.Equal(t, 100*time.Millisecond, b0)

	b1 := backoffDuration(1, 100*time.Millisecond, 5*time.Second, false)
	assert.Equal(t, 200*time.Millisecond, b1)

	b2 := backoffDuration(2, 100*time.Millisecond, 5*time.Second, false)
	assert.Equal(t, 400*time.Millisecond, b2)

	// Capped at max.
	b10 := backoffDuration(10, 100*time.Millisecond, 5*time.Second, false)
	assert.Equal(t, 5*time.Second, b10)
}

func TestBackoffDuration_WithJitter(t *testing.T) {
	b := backoffDuration(2, 100*time.Millisecond, 5*time.Second, true)
	// With jitter, result is [0, 400ms).
	assert.Less(t, b, 400*time.Millisecond)
	assert.GreaterOrEqual(t, b, time.Duration(0))
}

func TestIsRetryable(t *testing.T) {
	assert.True(t, IsRetryable(status.Error(codes.Unavailable, "")))
	assert.True(t, IsRetryable(status.Error(codes.DeadlineExceeded, "")))
	assert.True(t, IsRetryable(status.Error(codes.ResourceExhausted, "")))
	assert.False(t, IsRetryable(status.Error(codes.NotFound, "")))
	assert.False(t, IsRetryable(status.Error(codes.InvalidArgument, "")))
	assert.False(t, IsRetryable(status.Error(codes.PermissionDenied, "")))
	assert.False(t, IsRetryable(nil))
}

func TestWithRetry_Option(t *testing.T) {
	c := New(nil, WithRetry(RetryConfig{MaxAttempts: 5}))
	assert.True(t, c.opts.retryEnabled)
	assert.Equal(t, 5, c.opts.retry.MaxAttempts)
	assert.Equal(t, 100*time.Millisecond, c.opts.retry.InitialBackoff) // default
}
