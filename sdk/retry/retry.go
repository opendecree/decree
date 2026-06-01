// Package retry provides a generic exponential-backoff retry engine shared
// by the configclient and adminclient SDK modules.
package retry

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

// Config holds the parameters for the retry loop.
type Config struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	MaxAttempts int
	// InitialBackoff is the delay before the first retry.
	InitialBackoff time.Duration
	// MaxBackoff caps the backoff duration.
	MaxBackoff time.Duration
	// Jitter adds randomness to the backoff to avoid thundering herd.
	Jitter bool
	// RetryableCheck reports whether an error is retryable.
	RetryableCheck func(err error) bool
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
