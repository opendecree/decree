package configclient

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func sv(s string) *pb.TypedValue {
	return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: s}}
}

func iv(n int64) *pb.TypedValue {
	return &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: n}}
}

func fv(n float64) *pb.TypedValue {
	return &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: n}}
}

func bv(b bool) *pb.TypedValue {
	return &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: b}}
}

// --- Hand-rolled mock ---

// mockCall records a stubbed return for a method, with an optional request matcher.
type mockCall struct {
	match  func(args ...any) bool // nil means match-all
	result []any
}

type mockRPC struct {
	mu    sync.Mutex
	stubs map[string][]mockCall // method name → stubs (first match wins)
	calls map[string]int        // method name → call count
}

func (m *mockRPC) init() {
	if m.stubs == nil {
		m.stubs = make(map[string][]mockCall)
	}
	if m.calls == nil {
		m.calls = make(map[string]int)
	}
}

// on registers a stub. match may be nil (matches any request).
func (m *mockRPC) on(method string, match func(args ...any) bool, result ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	m.stubs[method] = append(m.stubs[method], mockCall{match: match, result: result})
}

// call finds the first matching stub and returns its result.
func (m *mockRPC) call(method string, args ...any) []any {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	m.calls[method]++
	for _, stub := range m.stubs[method] {
		if stub.match == nil || stub.match(args...) {
			return stub.result
		}
	}
	panic("mockRPC: no stub for " + method)
}

// called returns the number of times a method was called.
func (m *mockRPC) called(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls == nil {
		return 0
	}
	return m.calls[method]
}

func (m *mockRPC) GetConfig(ctx context.Context, in *pb.GetConfigRequest, opts ...grpc.CallOption) (*pb.GetConfigResponse, error) {
	r := m.call("GetConfig", in)
	return r[0].(*pb.GetConfigResponse), errOrNil(r[1])
}

func (m *mockRPC) GetField(ctx context.Context, in *pb.GetFieldRequest, opts ...grpc.CallOption) (*pb.GetFieldResponse, error) {
	r := m.call("GetField", in)
	return r[0].(*pb.GetFieldResponse), errOrNil(r[1])
}

func (m *mockRPC) GetFields(ctx context.Context, in *pb.GetFieldsRequest, opts ...grpc.CallOption) (*pb.GetFieldsResponse, error) {
	r := m.call("GetFields", in)
	return r[0].(*pb.GetFieldsResponse), errOrNil(r[1])
}

func (m *mockRPC) SetField(ctx context.Context, in *pb.SetFieldRequest, opts ...grpc.CallOption) (*pb.SetFieldResponse, error) {
	r := m.call("SetField", in)
	return r[0].(*pb.SetFieldResponse), errOrNil(r[1])
}

func (m *mockRPC) SetFields(ctx context.Context, in *pb.SetFieldsRequest, opts ...grpc.CallOption) (*pb.SetFieldsResponse, error) {
	r := m.call("SetFields", in)
	return r[0].(*pb.SetFieldsResponse), errOrNil(r[1])
}

func (m *mockRPC) ListVersions(ctx context.Context, in *pb.ListVersionsRequest, opts ...grpc.CallOption) (*pb.ListVersionsResponse, error) {
	r := m.call("ListVersions", in)
	return r[0].(*pb.ListVersionsResponse), errOrNil(r[1])
}

func (m *mockRPC) GetVersion(ctx context.Context, in *pb.GetVersionRequest, opts ...grpc.CallOption) (*pb.GetVersionResponse, error) {
	r := m.call("GetVersion", in)
	return r[0].(*pb.GetVersionResponse), errOrNil(r[1])
}

func (m *mockRPC) RollbackToVersion(ctx context.Context, in *pb.RollbackToVersionRequest, opts ...grpc.CallOption) (*pb.RollbackToVersionResponse, error) {
	r := m.call("RollbackToVersion", in)
	return r[0].(*pb.RollbackToVersionResponse), errOrNil(r[1])
}

func (m *mockRPC) Subscribe(ctx context.Context, in *pb.SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.SubscribeResponse], error) {
	r := m.call("Subscribe", in)
	return nil, errOrNil(r[1])
}

func (m *mockRPC) ExportConfig(ctx context.Context, in *pb.ExportConfigRequest, opts ...grpc.CallOption) (*pb.ExportConfigResponse, error) {
	r := m.call("ExportConfig", in)
	return r[0].(*pb.ExportConfigResponse), errOrNil(r[1])
}

func (m *mockRPC) ImportConfig(ctx context.Context, in *pb.ImportConfigRequest, opts ...grpc.CallOption) (*pb.ImportConfigResponse, error) {
	r := m.call("ImportConfig", in)
	return r[0].(*pb.ImportConfigResponse), errOrNil(r[1])
}

// errOrNil safely converts an any to error (handles typed nil).
func errOrNil(v any) error {
	if v == nil {
		return nil
	}
	return v.(error)
}

// --- Get ---

func TestGet_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("GetField", func(args ...any) bool {
		r := args[0].(*pb.GetFieldRequest)
		return r.TenantId == "t1" && r.FieldPath == "payments.fee"
	}, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "payments.fee", Value: sv("0.5%"), Checksum: "abc"},
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
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil,
		(*pb.GetFieldResponse)(nil), status.Error(codes.NotFound, "not found"))

	_, err := client.Get(ctx, "t1", "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want %v", err, ErrNotFound)
	}
}

// --- GetAll ---

func TestGetAll_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetConfig", nil, &pb.GetConfigResponse{
		Config: &pb.Config{
			TenantId: "t1",
			Version:  3,
			Values: []*pb.ConfigValue{
				{FieldPath: "a", Value: sv("1")},
				{FieldPath: "b", Value: sv("2")},
			},
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
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		return r.TenantId == "t1" && r.FieldPath == "a" && r.Value != nil && typedValueToString(r.Value) == "new"
	}, &pb.SetFieldResponse{}, nil)

	err := client.Set(ctx, "t1", "a", "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSet_Locked(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("SetField", nil,
		(*pb.SetFieldResponse)(nil), status.Error(codes.PermissionDenied, "locked"))

	err := client.Set(ctx, "t1", "a", "new")
	if !errors.Is(err, ErrLocked) {
		t.Errorf("got error %v, want %v", err, ErrLocked)
	}
}

// --- SetMany ---

func TestSetMany_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("SetFields", nil, &pb.SetFieldsResponse{}, nil)

	err := client.SetMany(ctx, "t1", map[string]string{"a": "1", "b": "2"}, "bulk update")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := rpc.called("SetFields"); n != 1 {
		t.Errorf("expected SetFields to be called once, got %d", n)
	}
}

// --- Snapshot ---

func TestSnapshot_PinnedVersion(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	// Snapshot resolves latest version
	rpc.on("GetConfig", func(args ...any) bool {
		r := args[0].(*pb.GetConfigRequest)
		return r.Version == nil
	}, &pb.GetConfigResponse{
		Config: &pb.Config{TenantId: "t1", Version: 5},
	}, nil)

	snap, err := client.Snapshot(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := snap.Version(); got != 5 {
		t.Errorf("got %v, want %v", got, int32(5))
	}

	// Subsequent read uses pinned version
	v := int32(5)
	rpc.on("GetField", func(args ...any) bool {
		r := args[0].(*pb.GetFieldRequest)
		return r.Version != nil && *r.Version == v
	}, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "a", Value: sv("val")},
	}, nil)

	val, err := snap.Get(ctx, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "val" {
		t.Errorf("got %v, want %v", val, "val")
	}
}

func TestAtVersion(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	snap := client.AtVersion("t1", 3)
	if got := snap.Version(); got != 3 {
		t.Errorf("got %v, want %v", got, int32(3))
	}

	v := int32(3)
	rpc.on("GetConfig", func(args ...any) bool {
		r := args[0].(*pb.GetConfigRequest)
		return r.Version != nil && *r.Version == v
	}, &pb.GetConfigResponse{
		Config: &pb.Config{TenantId: "t1", Version: 3, Values: []*pb.ConfigValue{
			{FieldPath: "x", Value: sv("y")},
		}},
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
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "a", Value: sv("old"), Checksum: "chk123"},
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

	// Write with checksum — add a new stub that matches
	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		return r.ExpectedChecksum != nil && *r.ExpectedChecksum == "chk123" && r.Value != nil && typedValueToString(r.Value) == "new"
	}, &pb.SetFieldResponse{}, nil)

	err = lv.Set(ctx, client, "new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetForUpdate_ChecksumMismatch(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "a", Value: sv("old"), Checksum: "stale"},
	}, nil)

	lv, err := client.GetForUpdate(ctx, "t1", "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rpc.on("SetField", nil,
		(*pb.SetFieldResponse)(nil), status.Error(codes.Aborted, "checksum mismatch"))

	err = lv.Set(ctx, client, "new")
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("got error %v, want %v", err, ErrChecksumMismatch)
	}
}

// --- Update ---

func TestUpdate_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "counter", Value: sv("5"), Checksum: "chk"},
	}, nil)

	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		return r.Value != nil && typedValueToString(r.Value) == "6" && r.ExpectedChecksum != nil && *r.ExpectedChecksum == "chk"
	}, &pb.SetFieldResponse{}, nil)

	err := client.Update(ctx, "t1", "counter", func(current string) (string, error) {
		// Simple increment
		return "6", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Typed getters ---

func TestGetInt_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "retries", Value: iv(42)},
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
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "name", Value: sv("hello")},
	}, nil)

	_, err := client.GetInt(ctx, "t1", "name")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("got error %v, want %v", err, ErrTypeMismatch)
	}
}

func TestGetFloat_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "rate", Value: fv(3.14)},
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
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "enabled", Value: bv(true)},
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
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "x", Value: iv(42)},
	}, nil)

	_, err := client.GetBool(ctx, "t1", "x")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("got error %v, want %v", err, ErrTypeMismatch)
	}
}

func TestGetString_AcceptsStringURLJSON(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	// string type
	rpc.on("GetField", func(args ...any) bool {
		r := args[0].(*pb.GetFieldRequest)
		return r.FieldPath == "s"
	}, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "s", Value: sv("hello")},
	}, nil)

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
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	// Value is nil (null)
	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "retries", Value: nil},
	}, nil)

	val, err := client.GetInt(ctx, "t1", "retries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 0 {
		t.Errorf("got %v, want %v", val, int64(0)) // zero value for null
	}
}

func TestGetIntNullable_Null(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "retries", Value: nil},
	}, nil)

	val, err := client.GetIntNullable(ctx, "t1", "retries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestGetIntNullable_HasValue(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "retries", Value: iv(5)},
	}, nil)

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
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "enabled", Value: nil},
	}, nil)

	val, err := client.GetBoolNullable(ctx, "t1", "enabled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestGetStringNullable_Null(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "name", Value: nil},
	}, nil)

	val, err := client.GetStringNullable(ctx, "t1", "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestGetStringNullable_EmptyString(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "name", Value: sv("")},
	}, nil)

	val, err := client.GetStringNullable(ctx, "t1", "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil {
		t.Fatal("expected non-nil")
	}
	if *val != "" {
		t.Errorf("got %v, want %v", *val, "") // empty string, not null
	}
}

// --- Typed setters ---

func TestSetInt_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		if r.Value == nil {
			return false
		}
		v, ok := r.Value.Kind.(*pb.TypedValue_IntegerValue)
		return ok && v.IntegerValue == 42
	}, &pb.SetFieldResponse{}, nil)

	if err := client.SetInt(ctx, "t1", "retries", 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetBool_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		if r.Value == nil {
			return false
		}
		v, ok := r.Value.Kind.(*pb.TypedValue_BoolValue)
		return ok && v.BoolValue
	}, &pb.SetFieldResponse{}, nil)

	if err := client.SetBool(ctx, "t1", "enabled", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetFloat_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		if r.Value == nil {
			return false
		}
		v, ok := r.Value.Kind.(*pb.TypedValue_NumberValue)
		return ok && v.NumberValue == 3.14
	}, &pb.SetFieldResponse{}, nil)

	if err := client.SetFloat(ctx, "t1", "rate", 3.14); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetNull_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		return r.Value == nil // null
	}, &pb.SetFieldResponse{}, nil)

	if err := client.SetNull(ctx, "t1", "retries"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
