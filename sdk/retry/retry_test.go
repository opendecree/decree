package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRetryableError(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	re := &RetryableError{Err: inner}

	if re.Error() != inner.Error() {
		t.Errorf("Error() = %q, want %q", re.Error(), inner.Error())
	}
	if !errors.Is(re, inner) {
		t.Error("errors.Is should find wrapped inner error via Unwrap")
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(&RetryableError{Err: fmt.Errorf("unavailable")}) {
		t.Error("expected RetryableError to be retryable")
	}
	if IsRetryable(fmt.Errorf("plain error")) {
		t.Error("expected plain error to not be retryable")
	}
	if IsRetryable(nil) {
		t.Error("expected nil to not be retryable")
	}
}

func TestConfig_WithDefaults(t *testing.T) {
	cfg := Config{}.WithDefaults()
	if cfg.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", cfg.MaxAttempts)
	}
	if cfg.InitialBackoff != 100*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 100ms", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 5*time.Second {
		t.Errorf("MaxBackoff = %v, want 5s", cfg.MaxBackoff)
	}
	if cfg.RetryableCheck == nil {
		t.Fatal("RetryableCheck should not be nil")
	}
	if !cfg.RetryableCheck(&RetryableError{Err: fmt.Errorf("transient")}) {
		t.Error("default RetryableCheck should accept RetryableError")
	}
}

// TestBackoffDuration_NegativeAttempt verifies that negative attempt values do
// not panic and are treated as attempt 0 (returning initial).
func TestBackoffDuration_NegativeAttempt(t *testing.T) {
	initial := 100 * time.Millisecond
	max := time.Second

	cases := []int{-1, -99}
	for _, attempt := range cases {
		got := BackoffDuration(attempt, initial, max, false)
		if got != initial {
			t.Errorf("attempt %d: got %v, want %v", attempt, got, initial)
		}
	}
}

func TestBackoffDuration_NormalProgression(t *testing.T) {
	initial := 100 * time.Millisecond
	max := 5 * time.Second

	// attempt 0: 100ms, attempt 1: 200ms, ..., caps at 5s
	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1600 * time.Millisecond,
		3200 * time.Millisecond,
		max, // 6400ms > max
	}
	for i, want := range expected {
		got := BackoffDuration(i, initial, max, false)
		if got != want {
			t.Errorf("attempt %d: got %v, want %v", i, got, want)
		}
	}
}

// TestBackoffDuration_HighAttemptsClamped checks that very high attempt counts
// (shifts 56-62 with initial=100ms) no longer wrap around to zero. Before the
// fix, these attempts produced 0s, creating a busy-loop instead of a 5s pause.
func TestBackoffDuration_HighAttemptsClamped(t *testing.T) {
	initial := 100 * time.Millisecond
	max := 5 * time.Second

	for attempt := 50; attempt <= 65; attempt++ {
		got := BackoffDuration(attempt, initial, max, false)
		if got != max {
			t.Errorf("attempt %d: got %v, want %v (overflow must clamp to max)", attempt, got, max)
		}
	}
}

// TestBackoffDuration_JitterBound checks that jitter stays within [0, backoff).
func TestBackoffDuration_JitterBound(t *testing.T) {
	initial := 100 * time.Millisecond
	max := 5 * time.Second

	for attempt := 0; attempt <= 10; attempt++ {
		noJitter := BackoffDuration(attempt, initial, max, false)
		for i := range 50 {
			got := BackoffDuration(attempt, initial, max, true)
			if got < 0 || (noJitter > 0 && got >= noJitter) {
				t.Errorf("attempt %d iter %d: jitter result %v out of [0, %v)", attempt, i, got, noJitter)
			}
		}
	}
}

// TestBackoffDuration_HighAttemptsJitter ensures jitter at overflow attempts
// stays within [0, max) and never returns zero due to wrap-around.
func TestBackoffDuration_HighAttemptsJitter(t *testing.T) {
	initial := 100 * time.Millisecond
	max := 5 * time.Second

	// With jitter the result is random in [0, max), so we cannot assert == max.
	// What we can assert is that it is non-negative (no wrap-around to negative)
	// and strictly less than max (jitter always reduces).
	for attempt := 56; attempt <= 62; attempt++ {
		for range 20 {
			got := BackoffDuration(attempt, initial, max, true)
			if got < 0 {
				t.Errorf("attempt %d: jitter produced negative duration %v", attempt, got)
			}
			if got >= max {
				t.Errorf("attempt %d: jitter result %v should be < max %v", attempt, got, max)
			}
		}
	}
}

// TestRun_Disabled verifies that Run calls fn exactly once when retry is off.
func TestRun_Disabled(t *testing.T) {
	calls := 0
	cfg := Config{}.WithDefaults()
	_, err := Run(context.Background(), false, cfg, func(_ context.Context) (int, error) {
		calls++
		return 0, fmt.Errorf("fail")
	})
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestConfig_WithDefaults_PreservesCustomValues(t *testing.T) {
	cfg := Config{
		MaxAttempts:    5,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
	}.WithDefaults()
	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", cfg.MaxAttempts)
	}
	if cfg.InitialBackoff != 200*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 200ms", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 10*time.Second {
		t.Errorf("MaxBackoff = %v, want 10s", cfg.MaxBackoff)
	}
}
