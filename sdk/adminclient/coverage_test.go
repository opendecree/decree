package adminclient

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestRetryDo_NilRetry covers retryDo when retry is disabled (default client).
func TestRetryDo_NilRetry(t *testing.T) {
	called := false
	c := &Client{}
	err := retryDo(context.Background(), c, func(_ context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected fn to be called")
	}
}

// TestRetryDo_WithRetry covers retryDo when retry is enabled and fn returns error.
func TestRetryDo_WithRetry(t *testing.T) {
	calls := 0
	c := &Client{opts: clientOptions{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			RetryableCheck: IsRetryable,
		},
	}}
	err := retryDo(context.Background(), c, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return &RetryableError{Err: fmt.Errorf("unavailable")}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
}

// TestRetryDo_PropagatesError covers retryDo returning an error after exhausting attempts.
func TestRetryDo_PropagatesError(t *testing.T) {
	sentinel := fmt.Errorf("always fails")
	c := &Client{opts: clientOptions{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    2,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
			RetryableCheck: IsRetryable,
		},
	}}
	err := retryDo(context.Background(), c, func(_ context.Context) error {
		return &RetryableError{Err: sentinel}
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("got error %v, want wrapping %v", err, sentinel)
	}
}

// TestRetry_ContextAlreadyCancelledBeforeBackoff covers the ctx.Err() != nil fast-path
// before the select in the retry loop (line 69-71 in retry.go). A pre-cancelled context
// with a long backoff ensures we hit the if-check rather than the select's ctx.Done() arm.
func TestRetry_ContextAlreadyCancelledBeforeBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so ctx.Err() is non-nil when we enter the backoff section.
	cancel()

	calls := 0
	c := &Client{opts: clientOptions{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    5,
			InitialBackoff: 10 * time.Second, // long enough that the ctx.Err() check fires first
			MaxBackoff:     10 * time.Second,
			RetryableCheck: IsRetryable,
		},
	}}

	_, err := retry(ctx, c, func(_ context.Context) (string, error) {
		calls++
		return "", &RetryableError{Err: fmt.Errorf("transient")}
	})

	if calls != 1 {
		t.Errorf("got %d calls, want 1", calls)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want context.Canceled", err)
	}
}

// TestRetry_ContextCancelledDuringBackoff covers the select's ctx.Done() arm in the retry loop.
// The context is live when fn runs but is cancelled while the backoff sleep is in progress.
func TestRetry_ContextCancelledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0
	c := &Client{opts: clientOptions{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    5,
			InitialBackoff: 10 * time.Second, // long backoff so select blocks
			MaxBackoff:     10 * time.Second,
			RetryableCheck: IsRetryable,
		},
	}}

	// Cancel the context from a goroutine after fn has returned but before backoff completes.
	go func() {
		// Give fn time to run and return the error, then cancel while select is waiting.
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	_, err := retry(ctx, c, func(_ context.Context) (string, error) {
		calls++
		return "", &RetryableError{Err: fmt.Errorf("transient")}
	})

	if calls != 1 {
		t.Errorf("got %d calls, want 1", calls)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want context.Canceled", err)
	}
}

// TestBackoffDuration_WithJitter covers the jitter branch in backoffDuration.
func TestBackoffDuration_WithJitter(t *testing.T) {
	// With jitter enabled the result must be in [0, backoff) where backoff = 100ms at attempt 0.
	d := backoffDuration(0, 100*time.Millisecond, 5*time.Second, true)
	if d < 0 || d >= 100*time.Millisecond {
		t.Errorf("jittered backoff %v out of expected range [0, 100ms)", d)
	}

	// Run several times to confirm it's not always zero (probabilistic, but 0 has P=1/1e9).
	nonZero := false
	for range 20 {
		if backoffDuration(0, 100*time.Millisecond, 5*time.Second, true) > 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("expected at least one non-zero jittered backoff in 20 trials")
	}
}

// TestListTenants_ErrorInPagination covers the error return inside the pagination loop.
func TestListTenants_ErrorInPagination(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	paginationErr := errors.New("page fetch failed")
	calls := 0
	ms.listTenantsFn = func(_ context.Context, _ *string, _ int32, pageToken string) (*ListTenantsResponse, error) {
		calls++
		if pageToken == "" {
			return &ListTenantsResponse{
				Tenants:       []*Tenant{{ID: "t1", Name: "a"}},
				NextPageToken: "page2",
			}, nil
		}
		return nil, paginationErr
	}

	_, err := client.ListTenants(context.Background(), "")
	if !errors.Is(err, paginationErr) {
		t.Errorf("got error %v, want %v", err, paginationErr)
	}
	if calls != 2 {
		t.Errorf("got %d calls, want 2", calls)
	}
}

// TestListConfigVersions_ErrorInPagination covers the error return inside the pagination loop.
func TestListConfigVersions_ErrorInPagination(t *testing.T) {
	mc := &mockConfigTransport{}
	client := New(WithConfigTransport(mc))

	paginationErr := errors.New("page fetch failed")
	calls := 0
	mc.listVersionsFn = func(_ context.Context, _ string, _ int32, pageToken string) (*ListVersionsResponse, error) {
		calls++
		if pageToken == "" {
			return &ListVersionsResponse{
				Versions:      []*Version{{Version: 3, CreatedAt: time.Now()}},
				NextPageToken: "page2",
			}, nil
		}
		return nil, paginationErr
	}

	_, err := client.ListConfigVersions(context.Background(), "t1")
	if !errors.Is(err, paginationErr) {
		t.Errorf("got error %v, want %v", err, paginationErr)
	}
	if calls != 2 {
		t.Errorf("got %d calls, want 2", calls)
	}
}

// TestGetLatestPublishedSchemaVersion_ServiceNotConfigured covers the nil schema guard.
func TestGetLatestPublishedSchemaVersion_ServiceNotConfigured(t *testing.T) {
	client := New() // no schema transport
	_, _, err := client.GetLatestPublishedSchemaVersion(context.Background(), "payments")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want ErrServiceNotConfigured", err)
	}
}
