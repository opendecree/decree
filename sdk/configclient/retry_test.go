package configclient

import (
	"context"
	"errors"
	"testing"
	"time"

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

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
	if calls != 1 {
		t.Errorf("should not retry when retry is disabled: got %d calls, want 1", calls)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("got %v, want %v", result, "ok")
	}
	if calls != 3 {
		t.Errorf("should have retried twice before succeeding: got %d calls, want 3", calls)
	}
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

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("should not retry NotFound: got %d calls, want 1", calls)
	}
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

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
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
	if calls != 1 {
		t.Errorf("got %d calls, want 1", calls)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want %v", err, context.Canceled)
	}
}

func TestRetryConfig_Defaults(t *testing.T) {
	cfg := RetryConfig{}.withDefaults()
	if cfg.MaxAttempts != 3 {
		t.Errorf("got %v, want %v", cfg.MaxAttempts, 3)
	}
	if cfg.InitialBackoff != 100*time.Millisecond {
		t.Errorf("got %v, want %v", cfg.InitialBackoff, 100*time.Millisecond)
	}
	if cfg.MaxBackoff != 5*time.Second {
		t.Errorf("got %v, want %v", cfg.MaxBackoff, 5*time.Second)
	}
	if cfg.RetryableCheck == nil {
		t.Fatal("expected non-nil RetryableCheck")
	}
}

func TestRetryConfig_PreservesCustomValues(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:    5,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
	}.withDefaults()
	if cfg.MaxAttempts != 5 {
		t.Errorf("got %v, want %v", cfg.MaxAttempts, 5)
	}
	if cfg.InitialBackoff != 200*time.Millisecond {
		t.Errorf("got %v, want %v", cfg.InitialBackoff, 200*time.Millisecond)
	}
	if cfg.MaxBackoff != 10*time.Second {
		t.Errorf("got %v, want %v", cfg.MaxBackoff, 10*time.Second)
	}
}

func TestBackoffDuration(t *testing.T) {
	// Without jitter, exponential: 100ms, 200ms, 400ms...
	b0 := backoffDuration(0, 100*time.Millisecond, 5*time.Second, false)
	if b0 != 100*time.Millisecond {
		t.Errorf("got %v, want %v", b0, 100*time.Millisecond)
	}

	b1 := backoffDuration(1, 100*time.Millisecond, 5*time.Second, false)
	if b1 != 200*time.Millisecond {
		t.Errorf("got %v, want %v", b1, 200*time.Millisecond)
	}

	b2 := backoffDuration(2, 100*time.Millisecond, 5*time.Second, false)
	if b2 != 400*time.Millisecond {
		t.Errorf("got %v, want %v", b2, 400*time.Millisecond)
	}

	// Capped at max.
	b10 := backoffDuration(10, 100*time.Millisecond, 5*time.Second, false)
	if b10 != 5*time.Second {
		t.Errorf("got %v, want %v", b10, 5*time.Second)
	}
}

func TestBackoffDuration_WithJitter(t *testing.T) {
	b := backoffDuration(2, 100*time.Millisecond, 5*time.Second, true)
	// With jitter, result is [0, 400ms).
	if b >= 400*time.Millisecond {
		t.Errorf("got %v, want < %v", b, 400*time.Millisecond)
	}
	if b < 0 {
		t.Errorf("got %v, want >= 0", b)
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(status.Error(codes.Unavailable, "")) {
		t.Error("expected Unavailable to be retryable")
	}
	if !IsRetryable(status.Error(codes.DeadlineExceeded, "")) {
		t.Error("expected DeadlineExceeded to be retryable")
	}
	if !IsRetryable(status.Error(codes.ResourceExhausted, "")) {
		t.Error("expected ResourceExhausted to be retryable")
	}
	if IsRetryable(status.Error(codes.NotFound, "")) {
		t.Error("expected NotFound to not be retryable")
	}
	if IsRetryable(status.Error(codes.InvalidArgument, "")) {
		t.Error("expected InvalidArgument to not be retryable")
	}
	if IsRetryable(status.Error(codes.PermissionDenied, "")) {
		t.Error("expected PermissionDenied to not be retryable")
	}
	if IsRetryable(nil) {
		t.Error("expected nil error to not be retryable")
	}
}

func TestWithRetry_Option(t *testing.T) {
	c := New(nil, WithRetry(RetryConfig{MaxAttempts: 5}))
	if !c.opts.retryEnabled {
		t.Error("expected retryEnabled to be true")
	}
	if c.opts.retry.MaxAttempts != 5 {
		t.Errorf("got %v, want %v", c.opts.retry.MaxAttempts, 5)
	}
	if c.opts.retry.InitialBackoff != 100*time.Millisecond {
		t.Errorf("got %v, want %v", c.opts.retry.InitialBackoff, 100*time.Millisecond) // default
	}
}
