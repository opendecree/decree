package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/storage/domain"
)

// --- GetFields ---

func TestGetFields_Success(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	cache.On("Get", mock.Anything, tenantID1, mock.Anything).Return(nil, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)

	val := "hello"
	chk := "abc"
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{
		FieldPath: "app.name", Value: &val, Checksum: &chk,
	}, nil)

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"app.name"},
	})
	require.NoError(t, err)
	assert.Len(t, resp.Values, 1)
	assert.Equal(t, "app.name", resp.Values[0].FieldPath)
}

func TestGetFields_SkipsMissing(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	cache.On("Get", mock.Anything, tenantID1, mock.Anything).Return(nil, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"missing"},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Values)
}

func TestGetFields_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.GetFields(context.Background(), &pb.GetFieldsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestGetFields_PreservesOrder verifies the per-field fan-out preserves
// request order regardless of completion order. Per-path mock matchers let us
// assert each value lands in the right slot.
func TestGetFields_PreservesOrder(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	cache.On("Get", mock.Anything, tenantID1, mock.Anything).Return(nil, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)

	paths := []string{"a.one", "a.two", "a.three", "a.four", "a.five"}
	for _, p := range paths {
		val := "v-" + p
		store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
			TenantID: tenantID1, FieldPath: p, Version: 1,
		}).Return(GetConfigValueAtVersionRow{FieldPath: p, Value: &val}, nil)
	}

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: paths,
	})
	require.NoError(t, err)
	require.Len(t, resp.Values, len(paths))
	for i, p := range paths {
		assert.Equal(t, p, resp.Values[i].FieldPath, "slot %d", i)
	}
}

// TestGetFields_MixedMissingAndPresent verifies that NotFound rows are
// dropped while present rows retain their request order.
func TestGetFields_MixedMissingAndPresent(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	cache.On("Get", mock.Anything, tenantID1, mock.Anything).Return(nil, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)

	present := "x"
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "have.first", Version: 1,
	}).Return(GetConfigValueAtVersionRow{FieldPath: "have.first", Value: &present}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "missing", Version: 1,
	}).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "have.second", Version: 1,
	}).Return(GetConfigValueAtVersionRow{FieldPath: "have.second", Value: &present}, nil)

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"have.first", "missing", "have.second"},
	})
	require.NoError(t, err)
	require.Len(t, resp.Values, 2)
	assert.Equal(t, "have.first", resp.Values[0].FieldPath)
	assert.Equal(t, "have.second", resp.Values[1].FieldPath)
}

// TestGetFields_PropagatesError verifies a non-NotFound store error
// surfaces as Internal and aborts the group.
func TestGetFields_PropagatesError(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	cache.On("Get", mock.Anything, tenantID1, mock.Anything).Return(nil, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	val := "ok"
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "ok.path", Version: 1,
	}).Return(GetConfigValueAtVersionRow{FieldPath: "ok.path", Value: &val}, nil).Maybe()
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "boom", Version: 1,
	}).Return(GetConfigValueAtVersionRow{}, errors.New("db exploded"))

	_, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"ok.path", "boom"},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// --- Slug resolution tests ---

func TestGetConfig_ByTenantName(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	// Resolve "acme" slug → tenant UUID
	store.On("GetTenantByName", mock.Anything, "acme").Return(domain.Tenant{
		ID: tenantID1, Name: "acme",
	}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{
		TenantID: tenantID1, Version: 1,
	}, nil)
	// Return a cached result so we skip the DB path
	cached := map[string]string{"key": "val"}
	cache.On("Get", mock.Anything, tenantID1, int32(1)).Return(cached, nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: "acme"})
	require.NoError(t, err)
	assert.Equal(t, tenantID1, resp.Config.TenantId)
	store.AssertCalled(t, "GetTenantByName", mock.Anything, "acme")
}

func TestGetConfig_ByTenantUUID(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	// UUID goes directly to version lookup — no name resolution
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{
		TenantID: tenantID1, Version: 1,
	}, nil)
	cached := map[string]string{"key": "val"}
	cache.On("Get", mock.Anything, tenantID1, int32(1)).Return(cached, nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})
	require.NoError(t, err)
	assert.Equal(t, tenantID1, resp.Config.TenantId)
	store.AssertNotCalled(t, "GetTenantByName", mock.Anything, mock.Anything)
}

func TestGetConfig_TenantNameNotFound(t *testing.T) {
	svc, store, _, _ := newTestService()
	store.On("GetTenantByName", mock.Anything, "nonexistent").Return(domain.Tenant{}, domain.ErrNotFound)

	_, err := svc.GetConfig(context.Background(), &pb.GetConfigRequest{TenantId: "nonexistent"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- SetFields ---

func TestSetFields_Success(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := context.Background()

	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.Anything).Return(domain.ConfigVersion{
		ID: versionID2, Version: 2, CreatedAt: time.Now(),
	}, nil)
	store.On("SetConfigValue", ctx, mock.Anything).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	resp, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{FieldPath: "app.name", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "test"}}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

func TestSetFields_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.SetFields(context.Background(), &pb.SetFieldsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- ListVersions ---

func TestListVersions_Success(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	store.On("ListConfigVersions", ctx, mock.Anything).Return([]domain.ConfigVersion{
		{ID: versionID2, Version: 2, CreatedAt: time.Now()},
		{ID: versionID3, Version: 1, CreatedAt: time.Now()},
	}, nil)

	resp, err := svc.ListVersions(ctx, &pb.ListVersionsRequest{TenantId: tenantID1})
	require.NoError(t, err)
	assert.Len(t, resp.Versions, 2)
}

func TestListVersions_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.ListVersions(context.Background(), &pb.ListVersionsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetVersion ---

func TestGetVersion_Success(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	store.On("GetConfigVersion", ctx, mock.Anything).Return(domain.ConfigVersion{
		ID: versionID2, Version: 2, CreatedBy: "admin", CreatedAt: time.Now(),
	}, nil)

	resp, err := svc.GetVersion(ctx, &pb.GetVersionRequest{TenantId: tenantID1, Version: 2})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

func TestGetVersion_NotFound(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	store.On("GetConfigVersion", ctx, mock.Anything).Return(domain.ConfigVersion{}, domain.ErrNotFound)

	_, err := svc.GetVersion(ctx, &pb.GetVersionRequest{TenantId: tenantID1, Version: 99})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- convert.go: typedValueToString coverage ---

func TestTypedValueToString_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		tv       *pb.TypedValue
		expected *string
	}{
		{"nil", nil, nil},
		{"string", &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hi"}}, strPtr("hi")},
		{"integer", &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}, strPtr("42")},
		{"number", &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 3.14}}, strPtr("3.14")},
		{"bool", &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}, strPtr("true")},
		{"url", &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://x.com"}}, strPtr("https://x.com")},
		{"json", &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{}`}}, strPtr("{}")},
		{"empty kind", &pb.TypedValue{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typedValueToString(tt.tv)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

// --- Subscribe ---

// mockServerStream implements grpc.ServerStreamingServer[pb.SubscribeResponse] for testing.
type mockServerStream struct {
	ctx  context.Context
	sent []*pb.SubscribeResponse
}

func (m *mockServerStream) Send(resp *pb.SubscribeResponse) error {
	m.sent = append(m.sent, resp)
	return nil
}

func (m *mockServerStream) Context() context.Context { return m.ctx }

func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}
func (m *mockServerStream) SendMsg(any) error            { return nil }
func (m *mockServerStream) RecvMsg(any) error            { return nil }

func TestSubscribe_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()

	stream := &mockServerStream{ctx: context.Background()}
	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: ""}, stream)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSubscribe_SubscribeError(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", ctx, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(nil), context.CancelFunc(func() {}), errors.New("subscribe failed"))

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSubscribe_ForwardsEvents(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent, 2)
	cancel := func() {}

	ctx, ctxCancel := context.WithCancel(context.Background())
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	now := time.Now()
	ch <- pubsub.ConfigChangeEvent{
		TenantID:  tenantID1,
		Version:   1,
		FieldPath: "app.name",
		OldValue:  "",
		NewValue:  "hello",
		ChangedBy: "admin",
		ChangedAt: now,
	}
	ch <- pubsub.ConfigChangeEvent{
		TenantID:  tenantID1,
		Version:   2,
		FieldPath: "app.port",
		OldValue:  "8080",
		NewValue:  "9090",
		ChangedBy: "admin",
		ChangedAt: now,
	}

	// Close the channel so the for loop exits after draining.
	close(ch)

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	require.NoError(t, err)

	require.Len(t, stream.sent, 2)
	assert.Equal(t, "app.name", stream.sent[0].Change.FieldPath)
	assert.Equal(t, int32(1), stream.sent[0].Change.Version)
	assert.Equal(t, "admin", stream.sent[0].Change.ChangedBy)
	assert.Equal(t, "app.port", stream.sent[1].Change.FieldPath)
	assert.Equal(t, int32(2), stream.sent[1].Change.Version)

	// No leak — cancel context just for cleanup.
	ctxCancel()
}

func TestSubscribe_FiltersByFieldPaths(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent, 3)
	cancel := func() {}

	ctx := context.Background()
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	now := time.Now()
	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 1, FieldPath: "app.name", NewValue: "v1", ChangedAt: now}
	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 2, FieldPath: "app.port", NewValue: "9090", ChangedAt: now}
	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 3, FieldPath: "app.name", NewValue: "v2", ChangedAt: now}
	close(ch)

	err := svc.Subscribe(&pb.SubscribeRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"app.name"},
	}, stream)
	require.NoError(t, err)

	// Only "app.name" events should be forwarded.
	require.Len(t, stream.sent, 2)
	assert.Equal(t, "app.name", stream.sent[0].Change.FieldPath)
	assert.Equal(t, "app.name", stream.sent[1].Change.FieldPath)
}

func TestSubscribe_ContextCancellation(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent) // unbuffered — blocks
	cancelCalled := false
	cancel := func() { cancelCalled = true }

	ctx, ctxCancel := context.WithCancel(context.Background())
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	// Cancel the context immediately so the select hits ctx.Done().
	ctxCancel()

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	require.NoError(t, err)
	assert.Empty(t, stream.sent)
	assert.True(t, cancelCalled, "subscriber cancel function should be called via defer")
}

func TestSubscribe_SendError(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent, 1)
	cancel := func() {}

	stream := &errServerStream{ctx: context.Background(), sendErr: errors.New("stream broken")}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 1, FieldPath: "x", NewValue: "y", ChangedAt: time.Now()}
	close(ch)

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream broken")
}

// errServerStream is a mock stream that returns an error on Send.
type errServerStream struct {
	ctx     context.Context
	sendErr error
}

func (m *errServerStream) Send(*pb.SubscribeResponse) error { return m.sendErr }
func (m *errServerStream) Context() context.Context         { return m.ctx }
func (m *errServerStream) SetHeader(metadata.MD) error      { return nil }
func (m *errServerStream) SendHeader(metadata.MD) error     { return nil }
func (m *errServerStream) SetTrailer(metadata.MD)           {}
func (m *errServerStream) SendMsg(any) error                { return nil }
func (m *errServerStream) RecvMsg(any) error                { return nil }
