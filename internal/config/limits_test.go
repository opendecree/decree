package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	assert.Equal(t, 1_000, l.MaxListLen)
	assert.Equal(t, 5*1024*1024, l.MaxDocBytes)
	assert.Equal(t, 1*1024*1024, l.MaxFieldValueBytes)
}

func newLimitedService(maxListLen int) *Service {
	store := &mockStore{}
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	return NewService(store, c, pub, sub,
		WithLogger(testLogger),
		WithLimits(Limits{MaxListLen: maxListLen}),
	)
}

func newImportLimitedService(limits Limits) (*Service, *mockStore) {
	store := &mockStore{}
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	svc := NewService(store, c, pub, sub,
		WithLogger(testLogger),
		WithLimits(limits),
	)
	return svc, store
}

func TestGetFields_ExceedsMaxListLen(t *testing.T) {
	svc := newLimitedService(2)

	_, err := svc.GetFields(superadminCtx(), &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"a", "b", "c"},
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "exceeds limit of 2")
}

func TestSetFields_ExceedsMaxListLen(t *testing.T) {
	svc := newLimitedService(2)

	_, err := svc.SetFields(superadminCtx(), &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{FieldPath: "a"},
			{FieldPath: "b"},
			{FieldPath: "c"},
		},
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "exceeds limit of 2")
}

func TestSubscribe_ExceedsMaxListLen(t *testing.T) {
	svc := newLimitedService(2)

	stream := &mockServerStream{ctx: superadminCtx()}
	err := svc.Subscribe(&pb.SubscribeRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"a", "b", "c"},
	}, stream)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "exceeds limit of 2")
}

func TestImportConfig_ExceedsMaxDocBytes(t *testing.T) {
	svc, _ := newImportLimitedService(Limits{MaxDocBytes: 10})

	_, err := svc.ImportConfig(superadminCtx(), &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: []byte(strings.Repeat("x", 11)),
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "exceeds limit of 10")
}

func TestImportConfig_ExceedsMaxFieldValueBytes(t *testing.T) {
	svc, store := newImportLimitedService(Limits{MaxFieldValueBytes: 5})

	store.On("GetTenantByID", superadminCtx(), tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", superadminCtx(), domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", superadminCtx(), schemaVersionID).
		Return([]domain.SchemaField{{Path: "app.name", FieldType: domain.FieldTypeString}}, nil)
	store.On("GetFieldLocks", superadminCtx(), tenantID1).
		Return([]domain.TenantFieldLock{}, nil)

	_, err := svc.ImportConfig(superadminCtx(), &pb.ImportConfigRequest{
		TenantId: tenantID1,
		YamlContent: []byte(`spec_version: "v1"
values:
  app.name:
    value: "toolongvalue"
`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "exceeds limit of 5")
}
