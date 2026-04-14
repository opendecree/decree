package schema

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

var now = time.Now()

func testSchema() domain.Schema {
	return domain.Schema{ID: testSchemaID, Name: "test-schema", CreatedAt: now, UpdatedAt: now}
}

func testVersion(v int32) domain.SchemaVersion {
	return domain.SchemaVersion{
		ID: testVersionID, SchemaID: testSchemaID, Version: v,
		Checksum: "abc", CreatedAt: now,
	}
}

func testTenant() domain.Tenant {
	return domain.Tenant{
		ID: testTenantID, Name: "acme", SchemaID: testSchemaID,
		SchemaVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
}

// --- ListSchemas ---

func TestListSchemas_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	store.On("ListSchemas", mock.Anything, mock.Anything).Return([]domain.Schema{
		testSchema(),
	}, nil)
	store.On("GetLatestSchemaVersion", mock.Anything, testSchemaID).Return(testVersion(1), nil)
	store.On("GetSchemaFields", mock.Anything, testVersionID).Return([]domain.SchemaField{}, nil)

	resp, err := svc.ListSchemas(context.Background(), &pb.ListSchemasRequest{PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Schemas, 1)
}

// --- DeleteSchema ---

func TestDeleteSchema_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	store.On("GetSchemaByID", mock.Anything, testSchemaID).Return(testSchema(), nil)
	store.On("DeleteSchema", mock.Anything, testSchemaID).Return(nil)

	_, err := svc.DeleteSchema(context.Background(), &pb.DeleteSchemaRequest{Id: testSchemaID})
	require.NoError(t, err)
}

func TestDeleteSchema_InvalidID(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	_, err := svc.DeleteSchema(context.Background(), &pb.DeleteSchemaRequest{Id: "bad"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetTenant ---

func TestGetTenant_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)

	resp, err := svc.GetTenant(context.Background(), &pb.GetTenantRequest{Id: testTenantID})
	require.NoError(t, err)
	assert.Equal(t, "acme", resp.Tenant.Name)
}

func TestGetTenant_NotFound(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	missingID := "99999999-9999-9999-9999-999999999999"
	store.On("GetTenantByID", mock.Anything, missingID).Return(domain.Tenant{}, domain.ErrNotFound)

	_, err := svc.GetTenant(context.Background(), &pb.GetTenantRequest{Id: missingID})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- ListTenants ---

func TestListTenants_BySchema(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	schemaID := testSchemaID
	store.On("ListTenantsBySchema", mock.Anything, mock.Anything).Return([]domain.Tenant{testTenant()}, nil)

	resp, err := svc.ListTenants(context.Background(), &pb.ListTenantsRequest{SchemaId: &schemaID, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Tenants, 1)
}

func TestListTenants_All(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	store.On("ListTenants", mock.Anything, mock.Anything).Return([]domain.Tenant{testTenant()}, nil)

	resp, err := svc.ListTenants(context.Background(), &pb.ListTenantsRequest{PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Tenants, 1)
}

// --- DeleteTenant ---

func TestDeleteTenant_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("DeleteTenant", mock.Anything, testTenantID).Return(nil)

	_, err := svc.DeleteTenant(context.Background(), &pb.DeleteTenantRequest{Id: testTenantID})
	require.NoError(t, err)
}

func TestDeleteTenant_InvalidID(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	_, err := svc.DeleteTenant(context.Background(), &pb.DeleteTenantRequest{Id: "bad"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- LockField ---

func TestLockField_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("CreateFieldLock", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.LockField(context.Background(), &pb.LockFieldRequest{
		TenantId:  testTenantID,
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
}

// --- UnlockField ---

func TestUnlockField_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("DeleteFieldLock", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.UnlockField(context.Background(), &pb.UnlockFieldRequest{
		TenantId:  testTenantID,
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
}

// --- ListFieldLocks ---

func TestListFieldLocks_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger, nil, nil)

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("GetFieldLocks", mock.Anything, testTenantID).Return([]domain.TenantFieldLock{
		{TenantID: testTenantID, FieldPath: "app.fee"},
	}, nil)

	resp, err := svc.ListFieldLocks(context.Background(), &pb.ListFieldLocksRequest{TenantId: testTenantID})
	require.NoError(t, err)
	assert.Len(t, resp.Locks, 1)
	assert.Equal(t, "app.fee", resp.Locks[0].FieldPath)
}
