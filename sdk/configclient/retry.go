package configclient

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
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
	// Default: false.
	Jitter bool
	// RetryableCheck determines if an error is retryable.
	// If nil, defaults to checking for [RetryableError].
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

// WithRetry enables automatic retry with exponential backoff for transient errors.
// By default, only errors wrapped in [RetryableError] by the transport are retried.
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

// retryDo executes fn with retries if retry is enabled, for void operations.
func retryDo(ctx context.Context, c *Client, fn func(ctx context.Context) error) error {
	_, err := retry(ctx, c, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
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
