// Package retry provides a generic exponential-backoff retry engine shared
// by the configclient and adminclient SDK modules.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"
)

// RetryableError wraps an error to indicate the operation may succeed on retry.
// Transport implementations should wrap transient errors (e.g., network issues,
// server overload) in RetryableError.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

// IsRetryable reports whether err is marked as retryable by the transport.
func IsRetryable(err error) bool {
	var re *RetryableError
	return errors.As(err, &re)
}

// Config holds the parameters for the retry loop.
type Config struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	// Default: 3.
	MaxAttempts int
	// InitialBackoff is the delay before the first retry.
	// Default: 100ms.
	InitialBackoff time.Duration
	// MaxBackoff caps the backoff duration.
	// Default: 5s.
	MaxBackoff time.Duration
	// Jitter adds randomness to the backoff to avoid thundering herd.
	// Default: false.
	Jitter bool
	// RetryableCheck reports whether an error is retryable.
	// If nil, defaults to checking for [RetryableError].
	RetryableCheck func(err error) bool
}

// WithDefaults returns a copy of c with zero fields replaced by sensible defaults.
func (c Config) WithDefaults() Config {
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

// Run executes fn with exponential-backoff retry when enabled is true.
// When enabled is false, fn is called exactly once.
// Only the algorithmic core (loop + backoff) lives here; callers supply
// RetryableCheck via cfg so there is no dependency on SDK error types.
func Run[T any](ctx context.Context, enabled bool, cfg Config, fn func(ctx context.Context) (T, error)) (T, error) {
	if !enabled {
		return fn(ctx)
	}

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

		backoff := BackoffDuration(attempt, cfg.InitialBackoff, cfg.MaxBackoff, cfg.Jitter)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return zero, lastErr
}

// RunDo is like Run but for void operations.
func RunDo(ctx context.Context, enabled bool, cfg Config, fn func(ctx context.Context) error) error {
	_, err := Run(ctx, enabled, cfg, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}

// BackoffDuration computes exponential backoff with optional jitter.
func BackoffDuration(attempt int, initial, max time.Duration, jitter bool) time.Duration {
	backoff := time.Duration(float64(initial) * math.Pow(2, float64(attempt)))
	if backoff > max {
		backoff = max
	}
	if jitter && backoff > 0 {
		backoff = time.Duration(rand.Int64N(int64(backoff)))
	}
	return backoff
}
