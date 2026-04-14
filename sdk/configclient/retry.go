package configclient

import (
	"context"
	"math"
	"math/rand/v2"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryConfig configures automatic retry with exponential backoff.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	// Default: 3.
	MaxAttempts int
	// InitialBackoff is the delay before the first retry.
	// Default: 100ms.
	InitialBackoff time.Duration
	// MaxBackoff caps the backoff duration.
	// Default: 5s.
	MaxBackoff time.Duration
	// Jitter adds randomness to backoff to avoid thundering herd.
	// Default: true.
	Jitter bool
	// RetryableCheck determines if an error is retryable.
	// If nil, defaults to retrying Unavailable, DeadlineExceeded,
	// and ResourceExhausted codes.
	RetryableCheck func(err error) bool
}

func (c RetryConfig) withDefaults() RetryConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = 100 * time.Millisecond
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 5 * time.Second
	}
	if c.RetryableCheck == nil {
		c.RetryableCheck = IsRetryable
	}
	return c
}

// WithRetry enables automatic retry with exponential backoff for transient
// gRPC errors. Only safe-to-retry errors are retried by default
// (Unavailable, DeadlineExceeded, ResourceExhausted).
//
// Example:
//
//	client := configclient.New(rpc, configclient.WithRetry(configclient.RetryConfig{}))
func WithRetry(cfg RetryConfig) Option {
	return func(o *options) {
		o.retry = cfg.withDefaults()
		o.retryEnabled = true
	}
}

// retry executes fn with retries if retry is enabled. Otherwise calls fn once.
func retry[T any](ctx context.Context, c *Client, fn func(ctx context.Context) (T, error)) (T, error) {
	if !c.opts.retryEnabled {
		return fn(ctx)
	}

	cfg := c.opts.retry
	var zero T
	var lastErr error

	for attempt := range cfg.MaxAttempts {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !cfg.RetryableCheck(err) {
			return zero, err
		}
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		if ctx.Err() != nil {
			return zero, ctx.Err()
		}

		backoff := backoffDuration(attempt, cfg.InitialBackoff, cfg.MaxBackoff, cfg.Jitter)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return zero, lastErr
}

// backoffDuration computes exponential backoff with optional jitter.
func backoffDuration(attempt int, initial, max time.Duration, jitter bool) time.Duration {
	backoff := time.Duration(float64(initial) * math.Pow(2, float64(attempt)))
	backoff = min(backoff, max)
	if jitter && backoff > 0 {
		backoff = time.Duration(rand.Int64N(int64(backoff)))
	}
	return backoff
}

// IsRetryable returns true for gRPC status codes that indicate a transient error.
func IsRetryable(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}
