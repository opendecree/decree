package configclient

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// --- Mock transport ---

type mockCall struct {
	match  func(args ...any) bool
	result []any
}

type mockTransport struct {
	mu    sync.Mutex
	stubs map[string][]mockCall
	calls map[string]int
}

func (m *mockTransport) init() {
	if m.stubs == nil {
		m.stubs = make(map[string][]mockCall)
	}
	if m.calls == nil {
		m.calls = make(map[string]int)
	}
}

func (m *mockTransport) on(method string, match func(args ...any) bool, result ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	m.stubs[method] = append(m.stubs[method], mockCall{match: match, result: result})
}

func (m *mockTransport) call(method string, args ...any) []any {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	m.calls[method]++
	for _, stub := range m.stubs[method] {
		if stub.match == nil || stub.match(args...) {
			return stub.result
		}
	}
	panic("mockTransport: no stub for " + method)
}

func (m *mockTransport) called(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls == nil {
		return 0
	}
	return m.calls[method]
}

func errOrNil(v any) error {
	if v == nil {
		return nil
	}
	return v.(error)
}

func (m *mockTransport) GetField(_ context.Context, req *GetFieldRequest) (*GetFieldResponse, error) {
	r := m.call("GetField", req)
	return r[0].(*GetFieldResponse), errOrNil(r[1])
}

func (m *mockTransport) GetConfig(_ context.Context, req *GetConfigRequest) (*GetConfigResponse, error) {
	r := m.call("GetConfig", req)
	return r[0].(*GetConfigResponse), errOrNil(r[1])
}

func (m *mockTransport) GetFields(_ context.Context, req *GetFieldsRequest) (*GetFieldsResponse, error) {
	r := m.call("GetFields", req)
	return r[0].(*GetFieldsResponse), errOrNil(r[1])
}

func (m *mockTransport) SetField(_ context.Context, req *SetFieldRequest) (*SetFieldResponse, error) {
	r := m.call("SetField", req)
	return r[0].(*SetFieldResponse), errOrNil(r[1])
}

func (m *mockTransport) SetFields(_ context.Context, req *SetFieldsRequest) (*SetFieldsResponse, error) {
	r := m.call("SetFields", req)
	return r[0].(*SetFieldsResponse), errOrNil(r[1])
}

func (m *mockTransport) Subscribe(_ context.Context, _ *SubscribeRequest) (Subscription, error) {
	r := m.call("Subscribe")
	return nil, errOrNil(r[0])
}

// --- Get ---

func TestGet_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", func(args ...any) bool {
		r := args[0].(*GetFieldRequest)
		return r.TenantID == "t1" && r.FieldPath == "payments.fee"
	}, &GetFieldResponse{
		FieldPath: "payments.fee", Value: StringVal("0.5%"), Checksum: "abc",
	}, nil)

	val, err := client.Get(ctx, "t1", "payments.fee")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "0.5%" {
		t.Errorf("got %v, want %v", val, "0.5%")
	}
}

func TestGet_NotFound(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, (*GetFieldResponse)(nil), ErrNotFound)

	_, err := client.Get(ctx, "t1", "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want %v", err, ErrNotFound)
	}
}

// --- GetAll ---

func TestGetAll_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetConfig", nil, &GetConfigResponse{
		TenantID: "t1",
		Version:  3,
		Values: []ConfigValue{
			{FieldPath: "a", Value: StringVal("1")},
			{FieldPath: "b", Value: StringVal("2")},
		},
	}, nil)

	vals, err := client.GetAll(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"a": "1", "b": "2"}
	if !reflect.DeepEqual(vals, want) {
		t.Errorf("got %v, want %v", vals, want)
	}
}

// --- Set ---

func TestSet_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.TenantID == "t1" && r.FieldPath == "a" && r.Value != nil && r.Value.String() == "new"
	}, &SetFieldResponse{}, nil)

	err := client.Set(ctx, "t1", "a", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSet_Locked(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", nil, (*SetFieldResponse)(nil), ErrLocked)

	err := client.Set(ctx, "t1", "a", "new")
	if !errors.Is(err, ErrLocked) {
		t.Errorf("got error %v, want %v", err, ErrLocked)
	}
	if errors.Is(err, ErrPermissionDenied) {
		t.Error("ErrLocked must not match ErrPermissionDenied")
	}
}

func TestSet_PermissionDenied(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", nil, (*SetFieldResponse)(nil), ErrPermissionDenied)

	err := client.Set(ctx, "t1", "a", "new")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("got error %v, want ErrPermissionDenied", err)
	}
	if errors.Is(err, ErrLocked) {
		t.Error("ErrPermissionDenied must not match ErrLocked")
	}
}

// --- SetMany ---

func TestSetMany_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetFields", nil, &SetFieldsResponse{}, nil)

	err := client.SetMany(ctx, "t1", map[string]string{"a": "1", "b": "2"}, "bulk update")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := tr.called("SetFields"); n != 1 {
		t.Errorf("expected SetFields to be called once, got %d", n)
	}
}

func TestSetManyTyped_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetFields", func(args ...any) bool {
		r := args[0].(*SetFieldsRequest)
		if r.TenantID != "t1" || r.Description != "typed bulk" || len(r.Updates) != 2 {
			return false
		}
		byPath := map[string]*TypedValue{}
		for _, u := range r.Updates {
			byPath[u.FieldPath] = u.Value
		}
		return byPath["count"] != nil && byPath["count"].Kind() == KindInteger && byPath["count"].MustIntValue() == 42 &&
			byPath["enabled"] != nil && byPath["enabled"].Kind() == KindBool && byPath["enabled"].MustBoolValue()
	}, &SetFieldsResponse{}, nil)

	err := client.SetManyTyped(ctx, "t1", map[string]*TypedValue{
		"count":   IntVal(42),
		"enabled": BoolVal(true),
	}, "typed bulk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := tr.called("SetFields"); n != 1 {
		t.Errorf("expected SetFields to be called once, got %d", n)
	}
}

// --- Snapshot ---

func TestSnapshot_PinnedVersion(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetConfig", func(args ...any) bool {
		r := args[0].(*GetConfigRequest)
		return r.Version == nil
	}, &GetConfigResponse{TenantID: "t1", Version: 5}, nil)

	snap, err := client.Snapshot(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := snap.Version(); got != 5 {
		t.Errorf("got %v, want %v", got, int32(5))
	}

	tr.on("GetField", func(args ...any) bool {
		r := args[0].(*GetFieldRequest)
		return r.Version != nil && *r.Version == 5
	}, &GetFieldResponse{FieldPath: "a", Value: StringVal("val")}, nil)

	val, err := snap.Get(ctx, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "val" {
		t.Errorf("got %v, want %v", val, "val")
	}
}

func TestAtVersion(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	snap := client.AtVersion("t1", 3)
	if got := snap.Version(); got != 3 {
		t.Errorf("got %v, want %v", got, int32(3))
	}

	v := int32(3)
	tr.on("GetConfig", func(args ...any) bool {
		r := args[0].(*GetConfigRequest)
		return r.Version != nil && *r.Version == v
	}, &GetConfigResponse{
		TenantID: "t1",
		Version:  3,
		Values:   []ConfigValue{{FieldPath: "x", Value: StringVal("y")}},
	}, nil)

	vals, err := snap.GetAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"x": "y"}
	if !reflect.DeepEqual(vals, want) {
		t.Errorf("got %v, want %v", vals, want)
	}
}

// --- Snapshot retry ---

func TestSnapshot_RetriedOnRetryableError(t *testing.T) {
	calls := 0
	tr := &mockTransport{}
	client := New(tr, WithRetry(RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		RetryableCheck: IsRetryable,
	}))
	ctx := context.Background()

	tr.on("GetConfig", func(args ...any) bool {
		calls++
		r := args[0].(*GetConfigRequest)
		return r.Version == nil
	}, &GetConfigResponse{TenantID: "t1", Version: 7}, nil)

	snap, err := client.Snapshot(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Version() != 7 {
		t.Errorf("got %v, want %v", snap.Version(), int32(7))
	}
}

func TestSnapshotGet_RetriedOnRetryableError(t *testing.T) {
	ctr := &countingReadTransport{failUntil: 2}
	client := New(ctr, WithRetry(RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		RetryableCheck: IsRetryable,
	}))
	ctx := context.Background()
	snap := client.AtVersion("t1", 4)

	val, err := snap.Get(ctx, "feature.x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "retried-ok" {
		t.Errorf("got %v, want retried-ok", val)
	}
	if ctr.calls != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", ctr.calls)
	}
}

func TestSnapshotGetAll_RetriedOnRetryableError(t *testing.T) {
	ctr := &countingReadTransport{failUntil: 2}
	client := New(ctr, WithRetry(RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		RetryableCheck: IsRetryable,
	}))
	ctx := context.Background()
	snap := client.AtVersion("t1", 2)

	vals, err := snap.GetAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vals["field"] != "retried-ok" {
		t.Errorf("got %v, want retried-ok", vals["field"])
	}
	if ctr.calls != 3 {
		t.Errorf("expected 3 calls, got %d", ctr.calls)
	}
}

func TestSnapshotGetFields_RetriedOnRetryableError(t *testing.T) {
	ctr := &countingReadTransport{failUntil: 2}
	client := New(ctr, WithRetry(RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		RetryableCheck: IsRetryable,
	}))
	ctx := context.Background()
	snap := client.AtVersion("t1", 2)

	vals, err := snap.GetFields(ctx, []string{"field"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vals["field"] != "retried-ok" {
		t.Errorf("got %v, want retried-ok", vals["field"])
	}
	if ctr.calls != 3 {
		t.Errorf("expected 3 calls, got %d", ctr.calls)
	}
}

// countingReadTransport fails the first N read calls with RetryableError and succeeds thereafter.
type countingReadTransport struct {
	mockTransport
	failUntil int
	calls     int
}

func (t *countingReadTransport) GetField(_ context.Context, _ *GetFieldRequest) (*GetFieldResponse, error) {
	t.calls++
	if t.calls <= t.failUntil {
		return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
	}
	return &GetFieldResponse{FieldPath: "feature.x", Value: StringVal("retried-ok")}, nil
}

func (t *countingReadTransport) GetConfig(_ context.Context, _ *GetConfigRequest) (*GetConfigResponse, error) {
	t.calls++
	if t.calls <= t.failUntil {
		return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
	}
	return &GetConfigResponse{
		TenantID: "t1",
		Version:  2,
		Values:   []ConfigValue{{FieldPath: "field", Value: StringVal("retried-ok")}},
	}, nil
}

func (t *countingReadTransport) GetFields(_ context.Context, _ *GetFieldsRequest) (*GetFieldsResponse, error) {
	t.calls++
	if t.calls <= t.failUntil {
		return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
	}
	return &GetFieldsResponse{
		Values: []ConfigValue{{FieldPath: "field", Value: StringVal("retried-ok")}},
	}, nil
}

// --- GetForUpdate + LockedValue.Set ---

func TestGetForUpdate_ThenSet(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "a", Value: StringVal("old"), Checksum: "chk123",
	}, nil)

	lv, err := client.GetForUpdate(ctx, "t1", "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lv.Value != "old" {
		t.Errorf("got %v, want %v", lv.Value, "old")
	}
	if lv.Checksum != "chk123" {
		t.Errorf("got %v, want %v", lv.Checksum, "chk123")
	}

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.ExpectedChecksum != nil && *r.ExpectedChecksum == "chk123" && r.Value != nil && r.Value.String() == "new"
	}, &SetFieldResponse{}, nil)

	err = lv.Set(ctx, "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetForUpdate_ChecksumMismatch(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "a", Value: StringVal("old"), Checksum: "stale",
	}, nil)

	lv, err := client.GetForUpdate(ctx, "t1", "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tr.on("SetField", nil, (*SetFieldResponse)(nil), ErrChecksumMismatch)

	err = lv.Set(ctx, "new")
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("got error %v, want %v", err, ErrChecksumMismatch)
	}
}

// TestLockedValue_CapturesClient verifies that the client is captured at
// GetForUpdate time so that Set() can invoke the remote call without the
// caller having to pass the client explicitly.
func TestLockedValue_CapturesClient(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", func(args ...any) bool {
		r := args[0].(*GetFieldRequest)
		return r.TenantID == "tenant-a" && r.FieldPath == "feature.flag"
	}, &GetFieldResponse{
		FieldPath: "feature.flag", Value: StringVal("off"), Checksum: "sum42",
	}, nil)

	lv, err := client.GetForUpdate(ctx, "tenant-a", "feature.flag")
	if err != nil {
		t.Fatalf("GetForUpdate: unexpected error: %v", err)
	}

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.TenantID == "tenant-a" &&
			r.FieldPath == "feature.flag" &&
			r.Value != nil && r.Value.String() == "on" &&
			r.ExpectedChecksum != nil && *r.ExpectedChecksum == "sum42"
	}, &SetFieldResponse{}, nil)

	// Set takes only ctx + new value — no client argument required.
	if err := lv.Set(ctx, "on"); err != nil {
		t.Fatalf("Set: unexpected error: %v", err)
	}

	if n := tr.called("SetField"); n != 1 {
		t.Errorf("expected SetField to be called once, got %d", n)
	}
}

// --- Update ---

func TestUpdate_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "counter", Value: StringVal("5"), Checksum: "chk",
	}, nil)

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Value != nil && r.Value.String() == "6" && r.ExpectedChecksum != nil && *r.ExpectedChecksum == "chk"
	}, &SetFieldResponse{}, nil)

	err := client.Update(ctx, "t1", "counter", func(current string) (string, error) {
		return "6", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdate_WriteOptionsForwarded(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "k", Value: StringVal("old"), Checksum: "chk1",
	}, nil)

	var capturedReq *SetFieldRequest
	tr.on("SetField", func(args ...any) bool {
		capturedReq = args[0].(*SetFieldRequest)
		return true
	}, &SetFieldResponse{}, nil)

	err := client.Update(ctx, "t1", "k", func(current string) (string, error) {
		return "new", nil
	}, WithDescription("my-desc"), WithValueDescription("val-desc"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq == nil {
		t.Fatal("SetField was not called")
	}
	if capturedReq.Description != "my-desc" {
		t.Errorf("description: got %q, want %q", capturedReq.Description, "my-desc")
	}
	if capturedReq.ValueDescription != "val-desc" {
		t.Errorf("value description: got %q, want %q", capturedReq.ValueDescription, "val-desc")
	}
}

func TestUpdate_RetryOnChecksumMismatch(t *testing.T) {
	ctx := context.Background()

	var stGets, stSets int
	st := &sequencedTransport{
		getField: func(_ context.Context, req *GetFieldRequest) (*GetFieldResponse, error) {
			stGets++
			switch stGets {
			case 1:
				return &GetFieldResponse{FieldPath: req.FieldPath, Value: StringVal("old"), Checksum: "chk1"}, nil
			default:
				return &GetFieldResponse{FieldPath: req.FieldPath, Value: StringVal("mid"), Checksum: "chk2"}, nil
			}
		},
		setField: func(_ context.Context, _ *SetFieldRequest) (*SetFieldResponse, error) {
			stSets++
			if stSets == 1 {
				// First attempt: concurrent writer changed the value.
				return nil, ErrChecksumMismatch
			}
			return &SetFieldResponse{}, nil
		},
	}

	client := New(st)
	err := client.Update(ctx, "t1", "field", func(current string) (string, error) {
		return current + "-updated", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stGets != 2 {
		t.Errorf("expected 2 GetField calls (1 per attempt), got %d", stGets)
	}
	if stSets != 2 {
		t.Errorf("expected 2 SetField calls (1 mismatch + 1 success), got %d", stSets)
	}
}

func TestUpdate_ChecksumMismatchExhaustsRetries(t *testing.T) {
	ctx := context.Background()

	var stGets, stSets int
	st := &sequencedTransport{
		getField: func(_ context.Context, req *GetFieldRequest) (*GetFieldResponse, error) {
			stGets++
			return &GetFieldResponse{FieldPath: req.FieldPath, Value: StringVal("v"), Checksum: "chk"}, nil
		},
		setField: func(_ context.Context, _ *SetFieldRequest) (*SetFieldResponse, error) {
			stSets++
			return nil, ErrChecksumMismatch
		},
	}

	client := New(st)
	err := client.Update(ctx, "t1", "field", func(current string) (string, error) {
		return current + "!", nil
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("got error %v, want ErrChecksumMismatch", err)
	}
	if stGets != updateMaxAttempts {
		t.Errorf("expected %d GetField calls, got %d", updateMaxAttempts, stGets)
	}
	if stSets != updateMaxAttempts {
		t.Errorf("expected %d SetField calls, got %d", updateMaxAttempts, stSets)
	}
}

func TestUpdate_UpdateFnError(t *testing.T) {
	fnErr := errors.New("fn failed")
	calls := 0
	tr := &sequencedTransport{
		getField: func(_ context.Context, _ *GetFieldRequest) (*GetFieldResponse, error) {
			calls++
			return &GetFieldResponse{FieldPath: "f", Value: StringVal("v"), Checksum: "cs"}, nil
		},
		setField: func(_ context.Context, _ *SetFieldRequest) (*SetFieldResponse, error) {
			t.Fatal("Set should not be called when updateFn errors")
			return nil, nil
		},
	}
	client := New(tr)
	err := client.Update(context.Background(), "t1", "f", func(string) (string, error) {
		return "", fnErr
	})
	if !errors.Is(err, fnErr) {
		t.Errorf("expected fnErr, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 GetField call, got %d", calls)
	}
}

func TestUpdate_GetForUpdateError(t *testing.T) {
	getErr := errors.New("get failed")
	tr := &sequencedTransport{
		getField: func(_ context.Context, _ *GetFieldRequest) (*GetFieldResponse, error) {
			return nil, getErr
		},
		setField: func(_ context.Context, _ *SetFieldRequest) (*SetFieldResponse, error) {
			t.Fatal("Set should not be called when GetForUpdate errors")
			return nil, nil
		},
	}
	client := New(tr)
	err := client.Update(context.Background(), "t1", "f", func(s string) (string, error) {
		return s + "_new", nil
	})
	if !errors.Is(err, getErr) {
		t.Errorf("expected getErr, got %v", err)
	}
}

// sequencedTransport is a Transport implementation driven by user-supplied
// function fields. Used to build stateful call sequences in tests.
type sequencedTransport struct {
	getField func(context.Context, *GetFieldRequest) (*GetFieldResponse, error)
	setField func(context.Context, *SetFieldRequest) (*SetFieldResponse, error)
}

func (t *sequencedTransport) GetField(ctx context.Context, req *GetFieldRequest) (*GetFieldResponse, error) {
	return t.getField(ctx, req)
}

func (t *sequencedTransport) SetField(ctx context.Context, req *SetFieldRequest) (*SetFieldResponse, error) {
	return t.setField(ctx, req)
}

func (t *sequencedTransport) GetConfig(_ context.Context, _ *GetConfigRequest) (*GetConfigResponse, error) {
	panic("sequencedTransport: GetConfig not implemented")
}

func (t *sequencedTransport) GetFields(_ context.Context, _ *GetFieldsRequest) (*GetFieldsResponse, error) {
	panic("sequencedTransport: GetFields not implemented")
}

func (t *sequencedTransport) SetFields(_ context.Context, _ *SetFieldsRequest) (*SetFieldsResponse, error) {
	panic("sequencedTransport: SetFields not implemented")
}

func (t *sequencedTransport) Subscribe(_ context.Context, _ *SubscribeRequest) (Subscription, error) {
	panic("sequencedTransport: Subscribe not implemented")
}

// --- Typed getters ---

func TestGetInt_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "retries", Value: IntVal(42),
	}, nil)

	val, err := client.GetInt(ctx, "t1", "retries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("got %v, want %v", val, int64(42))
	}
}

func TestGetInt_TypeMismatch(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "name", Value: StringVal("hello"),
	}, nil)

	_, err := client.GetInt(ctx, "t1", "name")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("got error %v, want %v", err, ErrTypeMismatch)
	}
}

func TestGetFloat_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "rate", Value: FloatVal(3.14),
	}, nil)

	val, err := client.GetFloat(ctx, "t1", "rate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 3.14 {
		t.Errorf("got %v, want %v", val, 3.14)
	}
}

func TestGetBool_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "enabled", Value: BoolVal(true),
	}, nil)

	val, err := client.GetBool(ctx, "t1", "enabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !val {
		t.Error("expected true, got false")
	}
}

func TestGetBool_TypeMismatch(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: IntVal(42),
	}, nil)

	_, err := client.GetBool(ctx, "t1", "x")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("got error %v, want %v", err, ErrTypeMismatch)
	}
}

func TestGetString_AcceptsStringURLJSON(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", func(args ...any) bool {
		r := args[0].(*GetFieldRequest)
		return r.FieldPath == "s"
	}, &GetFieldResponse{FieldPath: "s", Value: StringVal("hello")}, nil)

	val, err := client.GetString(ctx, "t1", "s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Errorf("got %v, want %v", val, "hello")
	}
}

// --- Null handling ---

func TestGetInt_Null(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "retries", Value: nil}, nil)

	val, err := client.GetInt(ctx, "t1", "retries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 0 {
		t.Errorf("got %v, want %v", val, int64(0))
	}
}

func TestGetIntNullable_Null(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "retries", Value: nil}, nil)

	val, err := client.GetIntNullable(ctx, "t1", "retries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestGetIntNullable_HasValue(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "retries", Value: IntVal(5)}, nil)

	val, err := client.GetIntNullable(ctx, "t1", "retries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil {
		t.Fatal("expected non-nil")
	}
	if *val != 5 {
		t.Errorf("got %v, want %v", *val, int64(5))
	}
}

func TestGetBoolNullable_Null(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "enabled", Value: nil}, nil)

	val, err := client.GetBoolNullable(ctx, "t1", "enabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestGetStringNullable_Null(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "name", Value: nil}, nil)

	val, err := client.GetStringNullable(ctx, "t1", "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestGetStringNullable_EmptyString(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "name", Value: StringVal("")}, nil)

	val, err := client.GetStringNullable(ctx, "t1", "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil {
		t.Fatal("expected non-nil")
	}
	if *val != "" {
		t.Errorf("got %v, want %v", *val, "")
	}
}

func TestGetString_Null(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "name", Value: nil}, nil)

	val, err := client.GetString(ctx, "t1", "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("got %q, want empty string", val)
	}
}

func TestGetFloat_Null(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "rate", Value: nil}, nil)

	val, err := client.GetFloat(ctx, "t1", "rate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 0 {
		t.Errorf("got %v, want 0", val)
	}
}

func TestGetBool_Null(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "enabled", Value: nil}, nil)

	val, err := client.GetBool(ctx, "t1", "enabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != false {
		t.Errorf("got %v, want false", val)
	}
}

// --- Typed setters ---

func TestSetInt_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Value != nil && r.Value.Kind() == KindInteger && r.Value.MustIntValue() == 42
	}, &SetFieldResponse{}, nil)

	if err := client.SetInt(ctx, "t1", "retries", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetBool_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Value != nil && r.Value.Kind() == KindBool && r.Value.MustBoolValue()
	}, &SetFieldResponse{}, nil)

	if err := client.SetBool(ctx, "t1", "enabled", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetFloat_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Value != nil && r.Value.Kind() == KindNumber && r.Value.MustFloatValue() == 3.14
	}, &SetFieldResponse{}, nil)

	if err := client.SetFloat(ctx, "t1", "rate", 3.14); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetNull_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Value == nil
	}, &SetFieldResponse{}, nil)

	if err := client.SetNull(ctx, "t1", "retries"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Write retry semantics ---

func TestSet_NoRetryEvenWithRetryEnabled(t *testing.T) {
	calls := 0
	tr := &mockTransport{}
	client := New(tr, WithRetry(RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		RetryableCheck: IsRetryable,
	}))
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		calls++
		return true
	}, (*SetFieldResponse)(nil), &RetryableError{Err: fmt.Errorf("unavailable")})

	err := client.Set(ctx, "t1", "a", "v")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("Set must not retry without idempotency key: got %d calls, want 1", calls)
	}
}

func TestSetMany_NoRetryEvenWithRetryEnabled(t *testing.T) {
	calls := 0
	tr := &mockTransport{}
	client := New(tr, WithRetry(RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		RetryableCheck: IsRetryable,
	}))
	ctx := context.Background()

	tr.on("SetFields", func(args ...any) bool {
		calls++
		return true
	}, (*SetFieldsResponse)(nil), &RetryableError{Err: fmt.Errorf("unavailable")})

	err := client.SetMany(ctx, "t1", map[string]string{"a": "1"}, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("SetMany must not retry without idempotency key: got %d calls, want 1", calls)
	}
}

func TestSet_RetriesWithIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	tr := &countingWriteTransport{failUntil: 2}
	client := New(tr, WithRetry(RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		RetryableCheck: IsRetryable,
	}))

	err := client.Set(ctx, "t1", "a", "v", WithIdempotencyKey("key-123"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.calls != 3 {
		t.Errorf("Set with idempotency key should retry: got %d calls, want 3", tr.calls)
	}
}

func TestSet_IdempotencyKeyPassedToTransport(t *testing.T) {
	var gotKey string
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		gotKey = r.IdempotencyKey
		return true
	}, &SetFieldResponse{}, nil)

	if err := client.Set(ctx, "t1", "a", "v", WithIdempotencyKey("my-key")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKey != "my-key" {
		t.Errorf("got idempotency key %q, want %q", gotKey, "my-key")
	}
}

func TestSetMany_IdempotencyKeyPassedToTransport(t *testing.T) {
	var gotKey string
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetFields", func(args ...any) bool {
		r := args[0].(*SetFieldsRequest)
		gotKey = r.IdempotencyKey
		return true
	}, &SetFieldsResponse{}, nil)

	if err := client.SetMany(ctx, "t1", map[string]string{"a": "1"}, "", WithIdempotencyKey("batch-key")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKey != "batch-key" {
		t.Errorf("got idempotency key %q, want %q", gotKey, "batch-key")
	}
}

// countingWriteTransport is a minimal Transport that fails the first N SetField
// calls with a RetryableError and succeeds thereafter.
type countingWriteTransport struct {
	mockTransport
	failUntil int
	calls     int
}

func (t *countingWriteTransport) SetField(_ context.Context, _ *SetFieldRequest) (*SetFieldResponse, error) {
	t.calls++
	if t.calls <= t.failUntil {
		return nil, &RetryableError{Err: fmt.Errorf("unavailable")}
	}
	return &SetFieldResponse{}, nil
}

// --- WriteOption tests ---

func TestSet_WithDescription(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Description == "why" && r.ValueDescription == "what"
	}, &SetFieldResponse{}, nil)

	err := client.Set(ctx, "t1", "a", "v", WithDescription("why"), WithValueDescription("what"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSet_WithExpectedChecksum(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.ExpectedChecksum != nil && *r.ExpectedChecksum == "abc"
	}, &SetFieldResponse{}, nil)

	err := client.Set(ctx, "t1", "a", "v", WithExpectedChecksum("abc"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetMany_WithValueDescriptionsAndChecksums(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetFields", func(args ...any) bool {
		r := args[0].(*SetFieldsRequest)
		if len(r.Updates) != 2 {
			return false
		}
		byPath := map[string]FieldUpdate{}
		for _, u := range r.Updates {
			byPath[u.FieldPath] = u
		}
		aOK := byPath["a"].ValueDescription == "desc-a" &&
			byPath["a"].ExpectedChecksum != nil && *byPath["a"].ExpectedChecksum == "chk-a"
		bOK := byPath["b"].ValueDescription == "" && byPath["b"].ExpectedChecksum == nil
		return aOK && bOK
	}, &SetFieldsResponse{}, nil)

	err := client.SetMany(ctx, "t1",
		map[string]string{"a": "1", "b": "2"},
		"batch",
		WithValueDescriptions(map[string]string{"a": "desc-a"}),
		WithFieldChecksums(map[string]string{"a": "chk-a"}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLockedValue_Set_WithDescription(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: StringVal("old"), Checksum: "chk",
	}, nil)

	lv, err := client.GetForUpdate(ctx, "t1", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Description == "my reason" && r.ValueDescription == "my val desc" &&
			r.ExpectedChecksum != nil && *r.ExpectedChecksum == "chk"
	}, &SetFieldResponse{}, nil)

	err = lv.Set(ctx, "new", WithDescription("my reason"), WithValueDescription("my val desc"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLockedValue_Set_ForwardsIdempotencyKey(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: StringVal("old"), Checksum: "chk",
	}, nil)

	lv, err := client.GetForUpdate(ctx, "t1", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.IdempotencyKey == "idem-key-123" &&
			r.ExpectedChecksum != nil && *r.ExpectedChecksum == "chk"
	}, &SetFieldResponse{}, nil)

	err = lv.Set(ctx, "new", WithIdempotencyKey("idem-key-123"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLockedValue_Set_RejectsUnsupportedOptions(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	makeLockedValue := func(t *testing.T) *LockedValue {
		t.Helper()
		tr.on("GetField", nil, &GetFieldResponse{
			FieldPath: "x", Value: StringVal("old"), Checksum: "chk",
		}, nil)
		lv, err := client.GetForUpdate(ctx, "t1", "x")
		if err != nil {
			t.Fatalf("GetForUpdate: unexpected error: %v", err)
		}
		return lv
	}

	tests := []struct {
		name string
		opt  WriteOption
	}{
		{
			name: "WithExpectedChecksum",
			opt:  WithExpectedChecksum("some-checksum"),
		},
		{
			name: "WithValueDescriptions",
			opt:  WithValueDescriptions(map[string]string{"x": "desc"}),
		},
		{
			name: "WithFieldChecksums",
			opt:  WithFieldChecksums(map[string]string{"x": "chk2"}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lv := makeLockedValue(t)
			err := lv.Set(ctx, "new", tc.opt)
			if err == nil {
				t.Fatalf("expected ErrInvalidArgument for %s, got nil", tc.name)
			}
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("expected errors.Is(err, ErrInvalidArgument), got: %v", err)
			}
		})
	}
}
