package configclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func sp(s string) *string { return &s }

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

// --- Mock ---

type mockRPC struct {
	mock.Mock
}

func (m *mockRPC) GetConfig(ctx context.Context, in *pb.GetConfigRequest, opts ...grpc.CallOption) (*pb.GetConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetConfigResponse), args.Error(1)
}

func (m *mockRPC) GetField(ctx context.Context, in *pb.GetFieldRequest, opts ...grpc.CallOption) (*pb.GetFieldResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetFieldResponse), args.Error(1)
}

func (m *mockRPC) GetFields(ctx context.Context, in *pb.GetFieldsRequest, opts ...grpc.CallOption) (*pb.GetFieldsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetFieldsResponse), args.Error(1)
}

func (m *mockRPC) SetField(ctx context.Context, in *pb.SetFieldRequest, opts ...grpc.CallOption) (*pb.SetFieldResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.SetFieldResponse), args.Error(1)
}

func (m *mockRPC) SetFields(ctx context.Context, in *pb.SetFieldsRequest, opts ...grpc.CallOption) (*pb.SetFieldsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.SetFieldsResponse), args.Error(1)
}

func (m *mockRPC) ListVersions(ctx context.Context, in *pb.ListVersionsRequest, opts ...grpc.CallOption) (*pb.ListVersionsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ListVersionsResponse), args.Error(1)
}

func (m *mockRPC) GetVersion(ctx context.Context, in *pb.GetVersionRequest, opts ...grpc.CallOption) (*pb.GetVersionResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetVersionResponse), args.Error(1)
}

func (m *mockRPC) RollbackToVersion(ctx context.Context, in *pb.RollbackToVersionRequest, opts ...grpc.CallOption) (*pb.RollbackToVersionResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.RollbackToVersionResponse), args.Error(1)
}

func (m *mockRPC) Subscribe(ctx context.Context, in *pb.SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.SubscribeResponse], error) {
	args := m.Called(ctx, in)
	return nil, args.Error(1)
}

func (m *mockRPC) ExportConfig(ctx context.Context, in *pb.ExportConfigRequest, opts ...grpc.CallOption) (*pb.ExportConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ExportConfigResponse), args.Error(1)
}

func (m *mockRPC) ImportConfig(ctx context.Context, in *pb.ImportConfigRequest, opts ...grpc.CallOption) (*pb.ImportConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ImportConfigResponse), args.Error(1)
}

// --- Get ---

func TestGet_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.MatchedBy(func(r *pb.GetFieldRequest) bool {
		return r.TenantId == "t1" && r.FieldPath == "payments.fee"
	})).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "payments.fee", Value: sv("0.5%"), Checksum: "abc"},
	}, nil)

	val, err := client.Get(ctx, "t1", "payments.fee")
	require.NoError(t, err)
	assert.Equal(t, "0.5%", val)
}

func TestGet_NotFound(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).
		Return((*pb.GetFieldResponse)(nil), status.Error(codes.NotFound, "not found"))

	_, err := client.Get(ctx, "t1", "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

// --- GetAll ---

func TestGetAll_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetConfig", mock.Anything, mock.Anything).Return(&pb.GetConfigResponse{
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
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, vals)
}

// --- Set ---

func TestSet_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		return r.TenantId == "t1" && r.FieldPath == "a" && r.Value != nil && typedValueToString(r.Value) == "new"
	})).Return(&pb.SetFieldResponse{}, nil)

	err := client.Set(ctx, "t1", "a", "new")
	require.NoError(t, err)
}

func TestSet_Locked(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.Anything).
		Return((*pb.SetFieldResponse)(nil), status.Error(codes.PermissionDenied, "locked"))

	err := client.Set(ctx, "t1", "a", "new")
	assert.ErrorIs(t, err, ErrLocked)
}

// --- SetMany ---

func TestSetMany_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("SetFields", mock.Anything, mock.Anything).Return(&pb.SetFieldsResponse{}, nil)

	err := client.SetMany(ctx, "t1", map[string]string{"a": "1", "b": "2"}, "bulk update")
	require.NoError(t, err)
	rpc.AssertCalled(t, "SetFields", mock.Anything, mock.Anything)
}

// --- Snapshot ---

func TestSnapshot_PinnedVersion(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	// Snapshot resolves latest version
	rpc.On("GetConfig", mock.Anything, mock.MatchedBy(func(r *pb.GetConfigRequest) bool {
		return r.Version == nil
	})).Return(&pb.GetConfigResponse{
		Config: &pb.Config{TenantId: "t1", Version: 5},
	}, nil)

	snap, err := client.Snapshot(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, int32(5), snap.Version())

	// Subsequent read uses pinned version
	v := int32(5)
	rpc.On("GetField", mock.Anything, mock.MatchedBy(func(r *pb.GetFieldRequest) bool {
		return r.Version != nil && *r.Version == v
	})).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "a", Value: sv("val")},
	}, nil)

	val, err := snap.Get(ctx, "a")
	require.NoError(t, err)
	assert.Equal(t, "val", val)
}

func TestAtVersion(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	snap := client.AtVersion("t1", 3)
	assert.Equal(t, int32(3), snap.Version())

	v := int32(3)
	rpc.On("GetConfig", mock.Anything, mock.MatchedBy(func(r *pb.GetConfigRequest) bool {
		return r.Version != nil && *r.Version == v
	})).Return(&pb.GetConfigResponse{
		Config: &pb.Config{TenantId: "t1", Version: 3, Values: []*pb.ConfigValue{
			{FieldPath: "x", Value: sv("y")},
		}},
	}, nil)

	vals, err := snap.GetAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"x": "y"}, vals)
}

// --- GetForUpdate + LockedValue.Set ---

func TestGetForUpdate_ThenSet(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "a", Value: sv("old"), Checksum: "chk123"},
	}, nil)

	lv, err := client.GetForUpdate(ctx, "t1", "a")
	require.NoError(t, err)
	assert.Equal(t, "old", lv.Value)
	assert.Equal(t, "chk123", lv.Checksum)

	// Write with checksum
	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		return r.ExpectedChecksum != nil && *r.ExpectedChecksum == "chk123" && r.Value != nil && typedValueToString(r.Value) == "new"
	})).Return(&pb.SetFieldResponse{}, nil)

	err = lv.Set(ctx, client, "new")
	require.NoError(t, err)
}

func TestGetForUpdate_ChecksumMismatch(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "a", Value: sv("old"), Checksum: "stale"},
	}, nil)

	lv, err := client.GetForUpdate(ctx, "t1", "a")
	require.NoError(t, err)

	rpc.On("SetField", mock.Anything, mock.Anything).
		Return((*pb.SetFieldResponse)(nil), status.Error(codes.Aborted, "checksum mismatch"))

	err = lv.Set(ctx, client, "new")
	assert.ErrorIs(t, err, ErrChecksumMismatch)
}

// --- Update ---

func TestUpdate_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "counter", Value: sv("5"), Checksum: "chk"},
	}, nil)

	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		return r.Value != nil && typedValueToString(r.Value) == "6" && r.ExpectedChecksum != nil && *r.ExpectedChecksum == "chk"
	})).Return(&pb.SetFieldResponse{}, nil)

	err := client.Update(ctx, "t1", "counter", func(current string) (string, error) {
		// Simple increment
		return "6", nil
	})
	require.NoError(t, err)
}

// --- Typed getters ---

func TestGetInt_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "retries", Value: iv(42)},
	}, nil)

	val, err := client.GetInt(ctx, "t1", "retries")
	require.NoError(t, err)
	assert.Equal(t, int64(42), val)
}

func TestGetInt_TypeMismatch(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "name", Value: sv("hello")},
	}, nil)

	_, err := client.GetInt(ctx, "t1", "name")
	assert.ErrorIs(t, err, ErrTypeMismatch)
}

func TestGetFloat_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "rate", Value: fv(3.14)},
	}, nil)

	val, err := client.GetFloat(ctx, "t1", "rate")
	require.NoError(t, err)
	assert.Equal(t, 3.14, val)
}

func TestGetBool_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "enabled", Value: bv(true)},
	}, nil)

	val, err := client.GetBool(ctx, "t1", "enabled")
	require.NoError(t, err)
	assert.True(t, val)
}

func TestGetBool_TypeMismatch(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "x", Value: iv(42)},
	}, nil)

	_, err := client.GetBool(ctx, "t1", "x")
	assert.ErrorIs(t, err, ErrTypeMismatch)
}

func TestGetString_AcceptsStringURLJSON(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	// string type
	rpc.On("GetField", mock.Anything, mock.MatchedBy(func(r *pb.GetFieldRequest) bool {
		return r.FieldPath == "s"
	})).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "s", Value: sv("hello")},
	}, nil)

	val, err := client.GetString(ctx, "t1", "s")
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}

// --- Null handling ---

func TestGetInt_Null(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	// Value is nil (null)
	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "retries", Value: nil},
	}, nil)

	val, err := client.GetInt(ctx, "t1", "retries")
	require.NoError(t, err)
	assert.Equal(t, int64(0), val) // zero value for null
}

func TestGetIntNullable_Null(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "retries", Value: nil},
	}, nil)

	val, err := client.GetIntNullable(ctx, "t1", "retries")
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestGetIntNullable_HasValue(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "retries", Value: iv(5)},
	}, nil)

	val, err := client.GetIntNullable(ctx, "t1", "retries")
	require.NoError(t, err)
	require.NotNil(t, val)
	assert.Equal(t, int64(5), *val)
}

func TestGetBoolNullable_Null(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "enabled", Value: nil},
	}, nil)

	val, err := client.GetBoolNullable(ctx, "t1", "enabled")
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestGetStringNullable_Null(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "name", Value: nil},
	}, nil)

	val, err := client.GetStringNullable(ctx, "t1", "name")
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestGetStringNullable_EmptyString(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{FieldPath: "name", Value: sv("")},
	}, nil)

	val, err := client.GetStringNullable(ctx, "t1", "name")
	require.NoError(t, err)
	require.NotNil(t, val)
	assert.Equal(t, "", *val) // empty string, not null
}

// --- Typed setters ---

func TestSetInt_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		if r.Value == nil {
			return false
		}
		v, ok := r.Value.Kind.(*pb.TypedValue_IntegerValue)
		return ok && v.IntegerValue == 42
	})).Return(&pb.SetFieldResponse{}, nil)

	require.NoError(t, client.SetInt(ctx, "t1", "retries", 42))
}

func TestSetBool_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		if r.Value == nil {
			return false
		}
		v, ok := r.Value.Kind.(*pb.TypedValue_BoolValue)
		return ok && v.BoolValue
	})).Return(&pb.SetFieldResponse{}, nil)

	require.NoError(t, client.SetBool(ctx, "t1", "enabled", true))
}

func TestSetFloat_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		if r.Value == nil {
			return false
		}
		v, ok := r.Value.Kind.(*pb.TypedValue_NumberValue)
		return ok && v.NumberValue == 3.14
	})).Return(&pb.SetFieldResponse{}, nil)

	require.NoError(t, client.SetFloat(ctx, "t1", "rate", 3.14))
}

func TestSetNull_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc)
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		return r.Value == nil // null
	})).Return(&pb.SetFieldResponse{}, nil)

	require.NoError(t, client.SetNull(ctx, "t1", "retries"))
}
