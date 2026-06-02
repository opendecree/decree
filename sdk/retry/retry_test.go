package retry

import (
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
