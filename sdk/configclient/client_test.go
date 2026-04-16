package configclient

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
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

	err = lv.Set(ctx, client, "new")
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

	err = lv.Set(ctx, client, "new")
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("got error %v, want %v", err, ErrChecksumMismatch)
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

// --- Typed setters ---

func TestSetInt_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Value != nil && r.Value.Kind() == KindInteger && r.Value.IntValue() == 42
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
		return r.Value != nil && r.Value.Kind() == KindBool && r.Value.BoolValue()
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
		return r.Value != nil && r.Value.Kind() == KindNumber && r.Value.FloatValue() == 3.14
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
