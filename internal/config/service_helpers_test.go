package config

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/cache"
	"github.com/opendecree/decree/internal/pubsub"
	celpkg "github.com/opendecree/decree/internal/schema/cel"
	"github.com/opendecree/decree/internal/storage/domain"
)

func TestErrToStatus(t *testing.T) {
	notFound := errToStatus(domain.ErrNotFound, "missing", "boom")
	assert.Equal(t, codes.NotFound, status.Code(notFound))
	assert.Contains(t, notFound.Error(), "missing")

	other := errToStatus(errors.New("db down"), "missing", "boom")
	assert.Equal(t, codes.Internal, status.Code(other))
	assert.Contains(t, other.Error(), "boom")
}

func TestLookupFieldType(t *testing.T) {
	assert.Equal(t, domain.FieldTypeString, lookupFieldType(nil, "x"))
	types := map[string]domain.FieldType{"a.b": domain.FieldTypeInteger}
	assert.Equal(t, domain.FieldTypeInteger, lookupFieldType(types, "a.b"))
	assert.Equal(t, domain.FieldTypeString, lookupFieldType(types, "missing"))
}

func TestPBFieldTypeMap(t *testing.T) {
	assert.Nil(t, pbFieldTypeMap(nil))
	out := pbFieldTypeMap(map[string]domain.FieldType{"n": domain.FieldTypeInteger})
	require.Len(t, out, 1)
	assert.Equal(t, domain.FieldTypeInteger.ToProto(), out["n"])
}

func TestMapDependentRequiredErr(t *testing.T) {
	// A crossFieldError maps to InvalidArgument, ignoring the fallback.
	cfe := &crossFieldError{kind: crossFieldKindValidation, err: errors.New("rule failed")}
	got := mapDependentRequiredErr(cfe, func() error { return errors.New("fallback") })
	assert.Equal(t, codes.InvalidArgument, status.Code(got))
	assert.Contains(t, got.Error(), "rule failed")

	// A plain error defers to the fallback.
	fallback := status.Error(codes.Internal, "internal boom")
	got = mapDependentRequiredErr(errors.New("tx failed"), func() error { return fallback })
	assert.Equal(t, fallback, got)
}

func TestActorPtrAndGetActor(t *testing.T) {
	svc, _, _, _ := newTestService()

	// No claims → "unknown" / nil pointer.
	assert.Equal(t, "unknown", svc.getActor(context.Background()))
	assert.Nil(t, svc.actorPtr(context.Background()))

	// Claims with a subject → that subject.
	ctx := auth.ContextWithClaims(context.Background(),
		&auth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "alice"}})
	assert.Equal(t, "alice", svc.getActor(ctx))
	got := svc.actorPtr(ctx)
	require.NotNil(t, got)
	assert.Equal(t, "alice", *got)
}

func TestResolveVersion(t *testing.T) {
	ctx := context.Background()

	t.Run("explicit version skips store", func(t *testing.T) {
		svc, store, _, _ := newTestService()
		want := int32(7)
		v, err := svc.resolveVersion(ctx, tenantID1, &want)
		require.NoError(t, err)
		assert.Equal(t, int32(7), v)
		store.AssertNotCalled(t, "GetLatestConfigVersion")
	})

	t.Run("no versions yet returns zero", func(t *testing.T) {
		svc, store, _, _ := newTestService()
		store.On("GetLatestConfigVersion", ctx, tenantID1).
			Return(domain.ConfigVersion{}, domain.ErrNotFound)
		v, err := svc.resolveVersion(ctx, tenantID1, nil)
		require.NoError(t, err)
		assert.Equal(t, int32(0), v)
	})

	t.Run("store error maps to Internal", func(t *testing.T) {
		svc, store, _, _ := newTestService()
		store.On("GetLatestConfigVersion", ctx, tenantID1).
			Return(domain.ConfigVersion{}, errors.New("db down"))
		_, err := svc.resolveVersion(ctx, tenantID1, nil)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("success returns latest version", func(t *testing.T) {
		svc, store, _, _ := newTestService()
		store.On("GetLatestConfigVersion", ctx, tenantID1).
			Return(domain.ConfigVersion{Version: 12}, nil)
		v, err := svc.resolveVersion(ctx, tenantID1, nil)
		require.NoError(t, err)
		assert.Equal(t, int32(12), v)
	})
}

func TestCelArtifactsTenantBinding(t *testing.T) {
	ctx := context.Background()

	// Nil receiver → bare binding with just the ID.
	var nilArtifacts *celArtifacts
	tb, err := nilArtifacts.tenantBinding(ctx, nil, tenantID1)
	require.NoError(t, err)
	assert.Equal(t, celpkg.TenantBinding{ID: tenantID1}, tb)

	// getTenant unset → bare binding.
	tb, err = (&celArtifacts{}).tenantBinding(ctx, nil, tenantID1)
	require.NoError(t, err)
	assert.Equal(t, celpkg.TenantBinding{ID: tenantID1}, tb)

	// getTenant set → delegates.
	a := &celArtifacts{getTenant: func(context.Context, Store, string) (celpkg.TenantBinding, error) {
		return celpkg.TenantBinding{ID: tenantID1, Name: "acme"}, nil
	}}
	tb, err = a.tenantBinding(ctx, nil, tenantID1)
	require.NoError(t, err)
	assert.Equal(t, "acme", tb.Name)
}

func TestPublishChanges_PublishErrorIsSwallowed(t *testing.T) {
	svc, _, _, pub := newTestService()
	pub.On("Publish", mock.Anything, mock.Anything).Return(errors.New("broker down"))
	// A publish failure must not panic — it is logged and ignored.
	assert.NotPanics(t, func() {
		svc.publishChanges(context.Background(), tenantID1, 1,
			[]pubsub.FieldChange{{FieldPath: "a.b", OldValue: "old", NewValue: "new"}},
			"alice")
	})
	pub.AssertExpectations(t)
}

func TestWithIdempotencyCache(t *testing.T) {
	store := &mockStore{}
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	idc := cache.NewMemoryIdempotencyCache(context.Background(), 0)
	svc := NewService(store, c, pub, sub, WithLogger(testLogger), WithIdempotencyCache(idc))
	assert.NotNil(t, svc.idempotencyCache)
}

func TestGetField_IncludeDescription(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())
	desc := "the fee"
	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 2}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{FieldPath: "app.fee", Value: strPtr("0.5"), Description: &desc}, nil)

	resp, err := svc.GetField(ctx, &pb.GetFieldRequest{
		TenantId:           tenantID1,
		FieldPath:          "app.fee",
		IncludeDescription: true,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Value.Description)
	assert.Equal(t, "the fee", *resp.Value.Description)
}

func TestStringifyValue_AllBranches(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		ft      domain.FieldType
		want    string
		wantErr bool
	}{
		{"nil value", nil, domain.FieldTypeString, "", false},
		{"int to integer", 5, domain.FieldTypeInteger, "5", false},
		{"int64 to integer", int64(6), domain.FieldTypeInteger, "6", false},
		{"whole float to integer", float64(7), domain.FieldTypeInteger, "7", false},
		{"string to integer", "8", domain.FieldTypeInteger, "8", false},
		{"fractional float to integer errors", 7.5, domain.FieldTypeInteger, "", true},
		{"bool to integer errors", true, domain.FieldTypeInteger, "", true},
		{"int to number", 5, domain.FieldTypeNumber, "5", false},
		{"int64 to number", int64(6), domain.FieldTypeNumber, "6", false},
		{"float to number", 3.5, domain.FieldTypeNumber, "3.5", false},
		{"string to number", "9.1", domain.FieldTypeNumber, "9.1", false},
		{"bool to number errors", true, domain.FieldTypeNumber, "", true},
		{"bool to bool", true, domain.FieldTypeBool, "true", false},
		{"string to bool", "false", domain.FieldTypeBool, "false", false},
		{"int to bool errors", 1, domain.FieldTypeBool, "", true},
		{"string to json", `{"k":1}`, domain.FieldTypeJSON, `{"k":1}`, false},
		{"map to json", map[string]int{"k": 1}, domain.FieldTypeJSON, `{"k":1}`, false},
		{"unmarshalable to json errors", make(chan int), domain.FieldTypeJSON, "", true},
		{"string to url", "https://x.io", domain.FieldTypeURL, "https://x.io", false},
		{"non-string default uses Sprintf", 42, domain.FieldTypeURL, "42", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stringifyValue(tt.value, tt.ft)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
