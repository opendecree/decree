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

// Error returns the underlying error message.
// Returns "<nil>" if Err is nil.
func (e *RetryableError) Error() string {
	if e.Err == nil {
		return "<nil>"
	}
	return e.Err.Error()
}

// Unwrap returns the wrapped error for use with [errors.As] and [errors.Is].
// Returns nil if Err is nil.
func (e *RetryableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

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
	// The zero value (false) means no jitter; WithDefaults does not change it.
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

		// ctx.Done() check is inside the select below; no need to poll ctx.Err() here.
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
//
// The exponent is computed with an integer bit-shift. To avoid wrap-around
// overflow (which can produce a zero or negative duration that bypasses the
// max-duration clamp), the shift is pre-checked: if left-shifting initial by
// shift bits would exceed math.MaxInt64, the result is clamped to max directly
// without performing the shift.
func BackoffDuration(attempt int, initial, max time.Duration, jitter bool) time.Duration {
	shift := attempt
	if shift > 62 {
		shift = 62
	}

	// Guard against left-shift overflow. When initial << shift would exceed
	// int64 max, the Go runtime wraps the value (it can go negative or wrap to
	// zero), which bypasses the backoff > max and backoff < 0 clamps below.
	// Pre-check using the equivalent inequality initial > MaxInt64 >> shift.
	var backoff time.Duration
	if shift > 0 && initial > time.Duration(math.MaxInt64)>>shift {
		backoff = max
	} else {
		backoff = initial << shift
	}

	if backoff > max || backoff < 0 {
		backoff = max
	}
	if jitter && backoff > 0 {
		backoff = time.Duration(rand.Int64N(int64(backoff)))
	}
	return backoff
}
