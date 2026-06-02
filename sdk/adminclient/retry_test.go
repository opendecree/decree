package adminclient

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRetry_NoRetryByDefault(t *testing.T) {
	calls := 0
	c := &Client{}

	result, err := retry(context.Background(), c, func(_ context.Context) (string, error) {
		calls++
		return "", &RetryableError{Err: fmt.Errorf("unavailable")}
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
	c := &Client{opts: clientOptions{
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
			return "", &RetryableError{Err: fmt.Errorf("unavailable")}
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

func TestRetry_DoesNotRetryOnInvalidArgument(t *testing.T) {
	calls := 0
	c := &Client{opts: clientOptions{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			RetryableCheck: IsRetryable,
		},
	}}

	_, err := retry(context.Background(), c, func(_ context.Context) (string, error) {
		calls++
		return "", InvalidArgumentError("bad field")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("got error %v, want wrapping %v", err, ErrInvalidArgument)
	}
	if calls != 1 {
		t.Errorf("should not retry InvalidArgument: got %d calls, want 1", calls)
	}
}

func TestRetry_DoesNotRetryOnNotFound(t *testing.T) {
	calls := 0
	c := &Client{opts: clientOptions{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			RetryableCheck: IsRetryable,
		},
	}}

	_, err := retry(context.Background(), c, func(_ context.Context) (string, error) {
		calls++
		return "", ErrNotFound
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
	c := &Client{opts: clientOptions{
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
		return "", &RetryableError{Err: fmt.Errorf("always down")}
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
}

func TestRetry_ContextCancellationStopsRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	c := &Client{opts: clientOptions{
		retryEnabled: true,
		retry: RetryConfig{
			MaxAttempts:    10,
			InitialBackoff: time.Second,
			RetryableCheck: IsRetryable,
		},
	}}

	_, err := retry(ctx, c, func(_ context.Context) (string, error) {
		calls++
		return "", &RetryableError{Err: fmt.Errorf("down")}
	})

	if calls != 1 {
		t.Errorf("got %d calls, want 1", calls)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want %v", err, context.Canceled)
	}
}

func TestRetryConfig_Defaults(t *testing.T) {
	cfg := RetryConfig{}.WithDefaults()
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
	}.WithDefaults()
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

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(&RetryableError{Err: fmt.Errorf("unavailable")}) {
		t.Error("expected RetryableError to be retryable")
	}
	if IsRetryable(ErrNotFound) {
		t.Error("expected ErrNotFound to not be retryable")
	}
	if IsRetryable(ErrInvalidArgument) {
		t.Error("expected ErrInvalidArgument to not be retryable")
	}
	if IsRetryable(ErrFailedPrecondition) {
		t.Error("expected ErrFailedPrecondition to not be retryable")
	}
	if IsRetryable(nil) {
		t.Error("expected nil error to not be retryable")
	}
}

func TestWithRetry_Option(t *testing.T) {
	c := New(WithRetry(RetryConfig{MaxAttempts: 5}))
	if !c.opts.retryEnabled {
		t.Error("expected retryEnabled to be true")
	}
	if c.opts.retry.MaxAttempts != 5 {
		t.Errorf("got %v, want %v", c.opts.retry.MaxAttempts, 5)
	}
	if c.opts.retry.InitialBackoff != 100*time.Millisecond {
		t.Errorf("got %v, want %v", c.opts.retry.InitialBackoff, 100*time.Millisecond)
	}
}

func TestRetryableError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	re := &RetryableError{Err: inner}
	if re.Error() != inner.Error() {
		t.Errorf("got %q, want %q", re.Error(), inner.Error())
	}
	if !errors.Is(re, inner) {
		t.Error("errors.Is should find wrapped inner error")
	}
}

func TestBackoffDuration(t *testing.T) {
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

	b10 := backoffDuration(10, 100*time.Millisecond, 5*time.Second, false)
	if b10 != 5*time.Second {
		t.Errorf("got %v, want %v", b10, 5*time.Second)
	}
}

func TestRetry_VerifyChain_RetriesOnUnavailable(t *testing.T) {
	calls := 0
	ma := &mockAuditTransport{
		queryWriteLogFn: func(_ context.Context, _ *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
			calls++
			if calls < 3 {
				return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
			}
			return &QueryWriteLogResponse{}, nil
		},
	}
	c := New(
		WithAuditTransport(ma),
		WithRetry(RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)

	result, err := c.VerifyChain(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Errorf("expected OK chain, got breaks: %v", result.Breaks)
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
}

func TestRetry_ListFieldLocks_RetriesOnUnavailable(t *testing.T) {
	calls := 0
	ms := &mockSchemaTransport{
		listFieldLocksFn: func(_ context.Context, _ string) ([]FieldLock, error) {
			calls++
			if calls < 3 {
				return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
			}
			return []FieldLock{{TenantID: "t1", FieldPath: "app.fee"}}, nil
		},
	}
	c := New(
		WithSchemaTransport(ms),
		WithRetry(RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)

	locks, err := c.ListFieldLocks(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locks) != 1 {
		t.Errorf("got %d locks, want 1", len(locks))
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
}

func TestRetry_GetSchema_RetriesOnUnavailable(t *testing.T) {
	calls := 0
	ms := &mockSchemaTransport{
		getSchemaFn: func(_ context.Context, _ string, _ *int32) (*Schema, error) {
			calls++
			if calls < 3 {
				return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
			}
			return &Schema{ID: "s1", Name: "payments", Version: 1}, nil
		},
	}
	c := New(
		WithSchemaTransport(ms),
		WithRetry(RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)

	schema, err := c.GetSchema(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema.ID != "s1" {
		t.Errorf("got ID %q, want %q", schema.ID, "s1")
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
}

func TestRetry_GetSchema_NoRetryOnInvalidArgument(t *testing.T) {
	calls := 0
	ms := &mockSchemaTransport{
		getSchemaFn: func(_ context.Context, _ string, _ *int32) (*Schema, error) {
			calls++
			return nil, ErrInvalidArgument
		},
	}
	c := New(
		WithSchemaTransport(ms),
		WithRetry(RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
		}),
	)

	_, err := c.GetSchema(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("should not retry InvalidArgument: got %d calls, want 1", calls)
	}
}

func TestRetry_GetSchema_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	ms := &mockSchemaTransport{
		getSchemaFn: func(_ context.Context, _ string, _ *int32) (*Schema, error) {
			calls++
			return nil, &RetryableError{Err: fmt.Errorf("down")}
		},
	}
	c := New(
		WithSchemaTransport(ms),
		WithRetry(RetryConfig{
			MaxAttempts:    10,
			InitialBackoff: time.Second,
		}),
	)

	_, err := c.GetSchema(ctx, "s1")
	if calls != 1 {
		t.Errorf("got %d calls, want 1", calls)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want context.Canceled", err)
	}
}

// TestRetry_ListSchemas_PerPageRetry verifies that a retryable error on page 2
// retries only that page and does not restart from page 1.
func TestRetry_ListSchemas_PerPageRetry(t *testing.T) {
	// calls tracks the pageToken passed on each invocation.
	var tokensReceived []string
	ms := &mockSchemaTransport{
		listSchemasFn: func(_ context.Context, _ int32, pageToken string) (*ListSchemasResponse, error) {
			tokensReceived = append(tokensReceived, pageToken)
			switch pageToken {
			case "":
				return &ListSchemasResponse{
					Schemas:       []*Schema{{ID: "s1"}},
					NextPageToken: "page2",
				}, nil
			case "page2":
				// Fail twice, then succeed — verifying retries stay on page 2.
				if len(tokensReceived) < 4 {
					return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
				}
				return &ListSchemasResponse{Schemas: []*Schema{{ID: "s2"}}}, nil
			}
			return nil, fmt.Errorf("unexpected token %q", pageToken)
		},
	}
	c := New(
		WithSchemaTransport(ms),
		WithRetry(RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)

	schemas, err := c.ListSchemas(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schemas) != 2 {
		t.Errorf("got %d schemas, want 2", len(schemas))
	}
	// Page 1 fetched once; page 2 fetched 3 times (2 failures + 1 success).
	if len(tokensReceived) != 4 {
		t.Errorf("got %d calls, want 4 (1 for page1 + 3 for page2)", len(tokensReceived))
	}
	// The first call must be page 1, the rest must all be page 2 (no restart).
	if tokensReceived[0] != "" {
		t.Errorf("call 0: got token %q, want empty (page 1)", tokensReceived[0])
	}
	for i := 1; i < len(tokensReceived); i++ {
		if tokensReceived[i] != "page2" {
			t.Errorf("call %d: got token %q, want page2", i, tokensReceived[i])
		}
	}
}

// TestRetry_ListTenants_PerPageRetry verifies per-page retry for ListTenants.
func TestRetry_ListTenants_PerPageRetry(t *testing.T) {
	var tokensReceived []string
	ms := &mockSchemaTransport{
		listTenantsFn: func(_ context.Context, _ *string, _ int32, pageToken string) (*ListTenantsResponse, error) {
			tokensReceived = append(tokensReceived, pageToken)
			switch pageToken {
			case "":
				return &ListTenantsResponse{
					Tenants:       []*Tenant{{ID: "t1"}},
					NextPageToken: "page2",
				}, nil
			case "page2":
				if len(tokensReceived) < 4 {
					return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
				}
				return &ListTenantsResponse{Tenants: []*Tenant{{ID: "t2"}}}, nil
			}
			return nil, fmt.Errorf("unexpected token %q", pageToken)
		},
	}
	c := New(
		WithSchemaTransport(ms),
		WithRetry(RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)

	tenants, err := c.ListTenants(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tenants) != 2 {
		t.Errorf("got %d tenants, want 2", len(tenants))
	}
	if len(tokensReceived) != 4 {
		t.Errorf("got %d calls, want 4", len(tokensReceived))
	}
	if tokensReceived[0] != "" {
		t.Errorf("call 0: got token %q, want empty", tokensReceived[0])
	}
	for i := 1; i < len(tokensReceived); i++ {
		if tokensReceived[i] != "page2" {
			t.Errorf("call %d: got token %q, want page2", i, tokensReceived[i])
		}
	}
}

// TestRetry_ListConfigVersions_PerPageRetry verifies per-page retry for ListConfigVersions.
func TestRetry_ListConfigVersions_PerPageRetry(t *testing.T) {
	var tokensReceived []string
	mc := &mockConfigTransport{
		listVersionsFn: func(_ context.Context, _ string, _ int32, pageToken string) (*ListVersionsResponse, error) {
			tokensReceived = append(tokensReceived, pageToken)
			switch pageToken {
			case "":
				return &ListVersionsResponse{
					Versions:      []*Version{{Version: 2}},
					NextPageToken: "page2",
				}, nil
			case "page2":
				if len(tokensReceived) < 4 {
					return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
				}
				return &ListVersionsResponse{Versions: []*Version{{Version: 1}}}, nil
			}
			return nil, fmt.Errorf("unexpected token %q", pageToken)
		},
	}
	c := New(
		WithConfigTransport(mc),
		WithRetry(RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)

	versions, err := c.ListConfigVersions(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("got %d versions, want 2", len(versions))
	}
	if len(tokensReceived) != 4 {
		t.Errorf("got %d calls, want 4", len(tokensReceived))
	}
	if tokensReceived[0] != "" {
		t.Errorf("call 0: got token %q, want empty", tokensReceived[0])
	}
	for i := 1; i < len(tokensReceived); i++ {
		if tokensReceived[i] != "page2" {
			t.Errorf("call %d: got token %q, want page2", i, tokensReceived[i])
		}
	}
}

// TestRetry_QueryWriteLog_PerPageRetry verifies per-page retry for QueryWriteLog.
func TestRetry_QueryWriteLog_PerPageRetry(t *testing.T) {
	var tokensReceived []string
	ma := &mockAuditTransport{
		queryWriteLogFn: func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
			tokensReceived = append(tokensReceived, req.PageToken)
			switch req.PageToken {
			case "":
				return &QueryWriteLogResponse{
					Entries:       []*AuditEntry{{ID: "e1"}},
					NextPageToken: "page2",
				}, nil
			case "page2":
				if len(tokensReceived) < 4 {
					return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
				}
				return &QueryWriteLogResponse{Entries: []*AuditEntry{{ID: "e2"}}}, nil
			}
			return nil, fmt.Errorf("unexpected token %q", req.PageToken)
		},
	}
	c := New(
		WithAuditTransport(ma),
		WithRetry(RetryConfig{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)

	entries, err := c.QueryWriteLog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}
	if len(tokensReceived) != 4 {
		t.Errorf("got %d calls, want 4", len(tokensReceived))
	}
	if tokensReceived[0] != "" {
		t.Errorf("call 0: got token %q, want empty", tokensReceived[0])
	}
	for i := 1; i < len(tokensReceived); i++ {
		if tokensReceived[i] != "page2" {
			t.Errorf("call %d: got token %q, want page2", i, tokensReceived[i])
		}
	}
}
