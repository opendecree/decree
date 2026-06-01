package configclient

import (
	"context"
	"time"

	sdkretry "github.com/opendecree/decree/sdk/retry"
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
	return sdkretry.Run(ctx, c.opts.retryEnabled, toSharedConfig(c.opts.retry), fn)
}

// retryDo executes fn with retries if retry is enabled, for void operations.
func retryDo(ctx context.Context, c *Client, fn func(ctx context.Context) error) error {
	return sdkretry.RunDo(ctx, c.opts.retryEnabled, toSharedConfig(c.opts.retry), fn)
}

// backoffDuration computes exponential backoff with optional jitter.
// Exposed for tests.
func backoffDuration(attempt int, initial, max time.Duration, jitter bool) time.Duration {
	return sdkretry.BackoffDuration(attempt, initial, max, jitter)
}

func toSharedConfig(c RetryConfig) sdkretry.Config {
	return sdkretry.Config{
		MaxAttempts:    c.MaxAttempts,
		InitialBackoff: c.InitialBackoff,
		MaxBackoff:     c.MaxBackoff,
		Jitter:         c.Jitter,
		RetryableCheck: c.RetryableCheck,
	}
}
