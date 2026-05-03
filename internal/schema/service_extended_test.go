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
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/validation"
)

var now = time.Now()

func testSchema() domain.Schema {
	return domain.Schema{ID: testSchemaID, Name: "test-schema", CreatedAt: now, UpdatedAt: now}
}

func testVersion() domain.SchemaVersion {
	return domain.SchemaVersion{
		ID: testVersionID, SchemaID: testSchemaID, Version: 1,
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
	svc := NewService(store, WithLogger(testLogger))

	store.On("ListSchemas", mock.Anything, mock.Anything).Return([]domain.Schema{
		testSchema(),
	}, nil)
	store.On("GetLatestSchemaVersion", mock.Anything, testSchemaID).Return(testVersion(), nil)
	store.On("GetSchemaFields", mock.Anything, testVersionID).Return([]domain.SchemaField{}, nil)

	resp, err := svc.ListSchemas(context.Background(), &pb.ListSchemasRequest{PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Schemas, 1)
}

func TestListSchemas_Pagination(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	s1 := testSchema()
	s2 := testSchema()
	s2.ID = "22222222-2222-2222-2222-222222222222"
	s2.Name = "schema-2"

	v1 := testVersion()
	v2 := testVersion()
	v2.ID = "33333333-3333-3333-3333-333333333333"

	// Page 1: returns 2 results (pageSize+1) → has next page
	store.On("ListSchemas", mock.Anything, mock.MatchedBy(func(p ListSchemasParams) bool {
		return p.Offset == 0 && p.Limit == 2
	})).Return([]domain.Schema{s1, s2}, nil)
	store.On("GetLatestSchemaVersion", mock.Anything, s1.ID).Return(v1, nil)
	store.On("GetSchemaFields", mock.Anything, v1.ID).Return([]domain.SchemaField{}, nil)

	resp, err := svc.ListSchemas(context.Background(), &pb.ListSchemasRequest{PageSize: 1})
	require.NoError(t, err)
	assert.Len(t, resp.Schemas, 1)
	assert.NotEmpty(t, resp.NextPageToken, "expected next_page_token for full page")

	// Page 2: returns 1 result (< pageSize+1) → no next page
	store.On("ListSchemas", mock.Anything, mock.MatchedBy(func(p ListSchemasParams) bool {
		return p.Offset == 1 && p.Limit == 2
	})).Return([]domain.Schema{s2}, nil)
	store.On("GetLatestSchemaVersion", mock.Anything, s2.ID).Return(v2, nil)
	store.On("GetSchemaFields", mock.Anything, v2.ID).Return([]domain.SchemaField{}, nil)

	resp, err = svc.ListSchemas(context.Background(), &pb.ListSchemasRequest{
		PageSize:  1,
		PageToken: resp.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Schemas, 1)
	assert.Empty(t, resp.NextPageToken, "expected no next_page_token for last page")
}

func TestListSchemas_InvalidPageToken(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))

	_, err := svc.ListSchemas(context.Background(), &pb.ListSchemasRequest{PageToken: "garbage"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- DeleteSchema ---

func TestDeleteSchema_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetSchemaByID", mock.Anything, testSchemaID).Return(testSchema(), nil)
	store.On("DeleteSchema", mock.Anything, testSchemaID).Return(nil)

	_, err := svc.DeleteSchema(superadminCtx(), &pb.DeleteSchemaRequest{Id: testSchemaID})
	require.NoError(t, err)
}

func TestDeleteSchema_InvalidID(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	// "bad" is not a UUID, so resolveSchema tries name lookup.
	store.On("GetSchemaByName", mock.Anything, "bad").Return(domain.Schema{}, domain.ErrNotFound)
	_, err := svc.DeleteSchema(superadminCtx(), &pb.DeleteSchemaRequest{Id: "bad"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- GetTenant ---

func TestGetTenant_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)

	resp, err := svc.GetTenant(context.Background(), &pb.GetTenantRequest{Id: testTenantID})
	require.NoError(t, err)
	assert.Equal(t, "acme", resp.Tenant.Name)
}

func TestGetTenant_NotFound(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	missingID := "99999999-9999-9999-9999-999999999999"
	store.On("GetTenantByID", mock.Anything, missingID).Return(domain.Tenant{}, domain.ErrNotFound)

	_, err := svc.GetTenant(context.Background(), &pb.GetTenantRequest{Id: missingID})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- ListTenants ---

func TestListTenants_BySchema(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	schemaID := testSchemaID
	store.On("ListTenantsBySchema", mock.Anything, mock.Anything).Return([]domain.Tenant{testTenant()}, nil)

	resp, err := svc.ListTenants(context.Background(), &pb.ListTenantsRequest{SchemaId: &schemaID, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Tenants, 1)
}

func TestListTenants_All(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("ListTenants", mock.Anything, mock.Anything).Return([]domain.Tenant{testTenant()}, nil)

	resp, err := svc.ListTenants(context.Background(), &pb.ListTenantsRequest{PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Tenants, 1)
}

// --- DeleteTenant ---

func TestDeleteTenant_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("DeleteTenant", mock.Anything, testTenantID).Return(nil)

	_, err := svc.DeleteTenant(superadminCtx(), &pb.DeleteTenantRequest{Id: testTenantID})
	require.NoError(t, err)
}

func TestDeleteTenant_InvalidID(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByName", mock.Anything, "bad").Return(domain.Tenant{}, domain.ErrNotFound)
	_, err := svc.DeleteTenant(superadminCtx(), &pb.DeleteTenantRequest{Id: "bad"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- LockField ---

func TestLockField_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("CreateFieldLock", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.LockField(superadminCtx(), &pb.LockFieldRequest{
		TenantId:  testTenantID,
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
}

// --- UnlockField ---

func TestUnlockField_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("DeleteFieldLock", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.UnlockField(superadminCtx(), &pb.UnlockFieldRequest{
		TenantId:  testTenantID,
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
}

// --- ListFieldLocks ---

func TestListFieldLocks_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("GetFieldLocks", mock.Anything, testTenantID).Return([]domain.TenantFieldLock{
		{TenantID: testTenantID, FieldPath: "app.fee"},
	}, nil)

	resp, err := svc.ListFieldLocks(context.Background(), &pb.ListFieldLocksRequest{TenantId: testTenantID})
	require.NoError(t, err)
	assert.Len(t, resp.Locks, 1)
	assert.Equal(t, "app.fee", resp.Locks[0].FieldPath)
}

// otherTenantID is a tenant id distinct from testTenantID, used to scope
// the caller's claims for "denied" tests of tenant-scoped RPCs.
const otherTenantID = "00000000-0000-0000-0000-000000000999"

// outOfScopeAdminCtx returns a context with admin claims scoped to
// otherTenantID — i.e. NOT testTenantID. Used to verify lock RPCs reject
// callers without tenant access.
func outOfScopeAdminCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{otherTenantID},
	})
}

func TestLockField_DeniedForOutOfScopeAdmin(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)

	_, err := svc.LockField(outOfScopeAdminCtx(), &pb.LockFieldRequest{
		TenantId:  testTenantID,
		FieldPath: "app.fee",
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestUnlockField_DeniedForOutOfScopeAdmin(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)

	_, err := svc.UnlockField(outOfScopeAdminCtx(), &pb.UnlockFieldRequest{
		TenantId:  testTenantID,
		FieldPath: "app.fee",
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestListFieldLocks_DeniedForOutOfScopeAdmin(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)

	_, err := svc.ListFieldLocks(outOfScopeAdminCtx(), &pb.ListFieldLocksRequest{TenantId: testTenantID})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// --- UpdateTenant ---

func TestUpdateTenant_UpdateName_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	newName := "new-acme"
	updated := testTenant()
	updated.Name = newName

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("UpdateTenantName", mock.Anything, UpdateTenantNameParams{
		ID:   testTenantID,
		Name: newName,
	}).Return(updated, nil)

	resp, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id:   testTenantID,
		Name: &newName,
	})
	require.NoError(t, err)
	assert.Equal(t, newName, resp.Tenant.Name)
	store.AssertExpectations(t)
}

func TestUpdateTenant_UpdateSchemaVersion_Success(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	newVersion := int32(2)
	updated := testTenant()
	updated.SchemaVersion = newVersion

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("UpdateTenantSchemaVersion", mock.Anything, UpdateTenantSchemaVersionParams{
		ID:            testTenantID,
		SchemaVersion: newVersion,
	}).Return(updated, nil)

	resp, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id:            testTenantID,
		SchemaVersion: &newVersion,
	})
	require.NoError(t, err)
	assert.Equal(t, newVersion, resp.Tenant.SchemaVersion)
	store.AssertExpectations(t)
}

func TestUpdateTenant_UpdateBothNameAndVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	newName := "renamed"
	newVersion := int32(3)
	afterName := testTenant()
	afterName.Name = newName
	afterVersion := afterName
	afterVersion.SchemaVersion = newVersion

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("UpdateTenantName", mock.Anything, UpdateTenantNameParams{
		ID:   testTenantID,
		Name: newName,
	}).Return(afterName, nil)
	store.On("UpdateTenantSchemaVersion", mock.Anything, UpdateTenantSchemaVersionParams{
		ID:            testTenantID,
		SchemaVersion: newVersion,
	}).Return(afterVersion, nil)

	resp, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id:            testTenantID,
		Name:          &newName,
		SchemaVersion: &newVersion,
	})
	require.NoError(t, err)
	assert.Equal(t, newName, resp.Tenant.Name)
	assert.Equal(t, newVersion, resp.Tenant.SchemaVersion)
	store.AssertExpectations(t)
}

func TestUpdateTenant_NoFieldsUpdated_FetchesCurrent(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)

	resp, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id: testTenantID,
	})
	require.NoError(t, err)
	assert.Equal(t, "acme", resp.Tenant.Name)
	store.AssertExpectations(t)
}

func TestUpdateTenant_InvalidID(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	// "bad" is not a UUID, so resolveTenant tries name lookup.
	store.On("GetTenantByName", mock.Anything, "bad").Return(domain.Tenant{}, domain.ErrNotFound)
	_, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{Id: "bad"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestUpdateTenant_InvalidSlugName(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	badName := "NOT A SLUG!"
	_, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id:   testTenantID,
		Name: &badName,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestUpdateTenant_NameNotFound(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	newName := "new-name"
	store.On("UpdateTenantName", mock.Anything, mock.Anything).Return(domain.Tenant{}, domain.ErrNotFound)

	_, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id:   testTenantID,
		Name: &newName,
	})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestUpdateTenant_SchemaVersionNotFound(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	v := int32(99)
	store.On("UpdateTenantSchemaVersion", mock.Anything, mock.Anything).Return(domain.Tenant{}, domain.ErrNotFound)

	_, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id:            testTenantID,
		SchemaVersion: &v,
	})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestUpdateTenant_NoFieldsUpdated_NotFound(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	missingID := "99999999-9999-9999-9999-999999999999"
	store.On("GetTenantByID", mock.Anything, missingID).Return(domain.Tenant{}, domain.ErrNotFound)

	_, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id: missingID,
	})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// validatorStoreAdapter bridges schema's mockStore (with
// GetSchemaVersionParams) to validation.Store (which expects
// domain.SchemaVersionKey). Only methods exercised by ValidatorFactory
// are forwarded; the rest are unused in this test.
type validatorStoreAdapter struct{ s *mockStore }

func (a *validatorStoreAdapter) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	return a.s.GetTenantByID(ctx, id)
}

func (a *validatorStoreAdapter) GetSchemaVersion(ctx context.Context, k domain.SchemaVersionKey) (domain.SchemaVersion, error) {
	return a.s.GetSchemaVersion(ctx, GetSchemaVersionParams{SchemaID: k.SchemaID, Version: k.Version})
}

func (a *validatorStoreAdapter) GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	return a.s.GetSchemaFields(ctx, schemaVersionID)
}

func TestUpdateTenant_SchemaVersionInvalidatesCache(t *testing.T) {
	store := &mockStore{}
	factory := validation.NewValidatorFactory(&validatorStoreAdapter{s: store})
	svc := NewService(store, WithLogger(testLogger), WithValidators(factory))

	newVersion := int32(2)
	updated := testTenant()
	updated.SchemaVersion = newVersion

	store.On("GetTenantByID", mock.Anything, testTenantID).Return(testTenant(), nil)
	store.On("UpdateTenantSchemaVersion", mock.Anything, mock.Anything).Return(updated, nil)

	resp, err := svc.UpdateTenant(superadminCtx(), &pb.UpdateTenantRequest{
		Id:            testTenantID,
		SchemaVersion: &newVersion,
	})
	require.NoError(t, err)
	assert.Equal(t, newVersion, resp.Tenant.SchemaVersion)
	store.AssertExpectations(t)
}

// --- Slug resolution tests ---

func TestGetTenant_ByName(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetTenantByName", mock.Anything, "acme").Return(testTenant(), nil)

	resp, err := svc.GetTenant(context.Background(), &pb.GetTenantRequest{Id: "acme"})
	require.NoError(t, err)
	assert.Equal(t, testTenantID, resp.Tenant.Id)
}

func TestGetSchema_ByName(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetSchemaByName", mock.Anything, "test-schema").Return(testSchema(), nil)
	store.On("GetLatestSchemaVersion", mock.Anything, testSchemaID).Return(testVersion(), nil)
	store.On("GetSchemaFields", mock.Anything, mock.Anything).Return([]domain.SchemaField{}, nil)

	resp, err := svc.GetSchema(context.Background(), &pb.GetSchemaRequest{Id: "test-schema"})
	require.NoError(t, err)
	assert.Equal(t, "test-schema", resp.Schema.Name)
}

// --- ExportSchema ---

// --- ListTenants with auth filtering ---

func TestListTenants_SuperadminSeesAll(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role: auth.RoleSuperAdmin,
	})

	store.On("ListTenants", ctx, ListTenantsParams{
		Limit: 51, AllowedTenantIDs: nil,
	}).Return([]domain.Tenant{testTenant()}, nil)

	resp, err := svc.ListTenants(ctx, &pb.ListTenantsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Tenants, 1)
	store.AssertExpectations(t)
}

func TestListTenants_NonSuperadminFiltered(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	allowedIDs := []string{testTenantID}
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: allowedIDs,
	})

	// Store should receive AllowedTenantIDs — filtering happens at store level.
	store.On("ListTenants", ctx, ListTenantsParams{
		Limit: 51, AllowedTenantIDs: allowedIDs,
	}).Return([]domain.Tenant{testTenant()}, nil)

	resp, err := svc.ListTenants(ctx, &pb.ListTenantsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Tenants, 1)
	assert.Equal(t, testTenantID, resp.Tenants[0].Id)
	store.AssertExpectations(t)
}

func TestListTenants_BySchemaFiltered(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	allowedIDs := []string{testTenantID}
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleUser,
		TenantIDs: allowedIDs,
	})

	schemaID := testSchemaID
	store.On("ListTenantsBySchema", ctx, ListTenantsBySchemaParams{
		SchemaID: schemaID, Limit: 51, AllowedTenantIDs: allowedIDs,
	}).Return([]domain.Tenant{testTenant()}, nil)

	resp, err := svc.ListTenants(ctx, &pb.ListTenantsRequest{SchemaId: &schemaID})
	require.NoError(t, err)
	assert.Len(t, resp.Tenants, 1)
	store.AssertExpectations(t)
}

func TestListTenants_NoAuthContext_SeesAll(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background() // No auth claims — permissive.

	store.On("ListTenants", ctx, ListTenantsParams{
		Limit: 51, AllowedTenantIDs: nil,
	}).Return([]domain.Tenant{testTenant()}, nil)

	resp, err := svc.ListTenants(ctx, &pb.ListTenantsRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Tenants, 1)
	store.AssertExpectations(t)
}

func TestListTenants_InvalidPageToken(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{Role: auth.RoleSuperAdmin})

	_, err := svc.ListTenants(ctx, &pb.ListTenantsRequest{PageToken: "garbage"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- ExportSchema ---

func TestExportSchema_LatestVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(domain.Schema{ID: testSchemaID, Name: "test-schema"}, nil)
	store.On("GetLatestSchemaVersion", ctx, testSchemaID).
		Return(domain.SchemaVersion{ID: testVersionID, Version: 1, Checksum: "abc"}, nil)
	store.On("GetSchemaFields", ctx, testVersionID).
		Return([]domain.SchemaField{
			{Path: "app.name", FieldType: "string"},
		}, nil)

	resp, err := svc.ExportSchema(ctx, &pb.ExportSchemaRequest{Id: testSchemaID})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.YamlContent)
	assert.Contains(t, string(resp.YamlContent), "test-schema")
	assert.Contains(t, string(resp.YamlContent), "app.name")
	store.AssertExpectations(t)
}

func TestExportSchema_SpecificVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()
	v := int32(2)

	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(domain.Schema{ID: testSchemaID, Name: "versioned-schema"}, nil)
	store.On("GetSchemaVersion", ctx, GetSchemaVersionParams{SchemaID: testSchemaID, Version: 2}).
		Return(domain.SchemaVersion{ID: testVersionID, Version: 2, Checksum: "def"}, nil)
	store.On("GetSchemaFields", ctx, testVersionID).
		Return([]domain.SchemaField{
			{Path: "config.timeout", FieldType: "duration"},
		}, nil)

	resp, err := svc.ExportSchema(ctx, &pb.ExportSchemaRequest{Id: testSchemaID, Version: &v})
	require.NoError(t, err)
	assert.Contains(t, string(resp.YamlContent), "versioned-schema")
	assert.Contains(t, string(resp.YamlContent), "config.timeout")
	store.AssertExpectations(t)
}

func TestExportSchema_SchemaNotFound(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := context.Background()

	store.On("GetSchemaByID", ctx, testSchemaID).Return(domain.Schema{}, domain.ErrNotFound)

	_, err := svc.ExportSchema(ctx, &pb.ExportSchemaRequest{Id: testSchemaID})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestExportSchema_InvalidID(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	store.On("GetSchemaByName", mock.Anything, "bad").Return(domain.Schema{}, domain.ErrNotFound)
	_, err := svc.ExportSchema(context.Background(), &pb.ExportSchemaRequest{Id: "bad"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- ImportSchema ---

func validYAML(name string) []byte {
	return []byte("spec_version: v1\nname: " + name + "\nfields:\n  app.name:\n    type: string\n")
}

func TestImportSchema_NewSchema(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	store.On("GetSchemaByName", ctx, "my-schema").Return(domain.Schema{}, domain.ErrNotFound)
	store.On("CreateSchema", ctx, mock.AnythingOfType("schema.CreateSchemaParams")).
		Return(domain.Schema{ID: testSchemaID, Name: "my-schema"}, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("schema.CreateSchemaVersionParams")).
		Return(domain.SchemaVersion{ID: testVersionID, SchemaID: testSchemaID, Version: 1, Checksum: "abc"}, nil)
	store.On("CreateSchemaField", ctx, mock.AnythingOfType("schema.CreateSchemaFieldParams")).
		Return(domain.SchemaField{Path: "app.name", FieldType: "string"}, nil)

	resp, err := svc.ImportSchema(ctx, &pb.ImportSchemaRequest{
		YamlContent: validYAML("my-schema"),
	})
	require.NoError(t, err)
	assert.Equal(t, "my-schema", resp.Schema.Name)
	assert.Equal(t, int32(1), resp.Schema.Version)
	store.AssertExpectations(t)
}

func TestImportSchema_NewSchemaWithAutoPublish(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	store.On("GetSchemaByName", ctx, "pub-schema").Return(domain.Schema{}, domain.ErrNotFound)
	store.On("CreateSchema", ctx, mock.AnythingOfType("schema.CreateSchemaParams")).
		Return(domain.Schema{ID: testSchemaID, Name: "pub-schema"}, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("schema.CreateSchemaVersionParams")).
		Return(domain.SchemaVersion{ID: testVersionID, SchemaID: testSchemaID, Version: 1, Checksum: "abc"}, nil)
	store.On("CreateSchemaField", ctx, mock.AnythingOfType("schema.CreateSchemaFieldParams")).
		Return(domain.SchemaField{Path: "app.name", FieldType: "string"}, nil)
	// autoPublish calls PublishSchema which calls GetSchemaByID + PublishSchemaVersion + GetSchemaFields.
	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(domain.Schema{ID: testSchemaID, Name: "pub-schema"}, nil)
	store.On("PublishSchemaVersion", ctx, PublishSchemaVersionParams{SchemaID: testSchemaID, Version: 1}).
		Return(domain.SchemaVersion{ID: testVersionID, Version: 1, Published: true}, nil)
	store.On("GetSchemaFields", ctx, testVersionID).
		Return([]domain.SchemaField{{Path: "app.name", FieldType: "string"}}, nil)

	resp, err := svc.ImportSchema(ctx, &pb.ImportSchemaRequest{
		YamlContent: validYAML("pub-schema"),
		AutoPublish: true,
	})
	require.NoError(t, err)
	assert.True(t, resp.Schema.Published)
	store.AssertExpectations(t)
}

func TestImportSchema_ExistingSchemaNewVersion(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	existingSchema := domain.Schema{ID: testSchemaID, Name: "my-schema"}
	latestVersion := domain.SchemaVersion{
		ID: "00000000-0000-0000-0000-000000000010", SchemaID: testSchemaID,
		Version: 1, Checksum: "different-checksum",
	}
	newVersionID := "00000000-0000-0000-0000-000000000011"

	store.On("GetSchemaByName", ctx, "my-schema").Return(existingSchema, nil)
	store.On("GetLatestSchemaVersion", ctx, testSchemaID).Return(latestVersion, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("schema.CreateSchemaVersionParams")).
		Return(domain.SchemaVersion{ID: newVersionID, SchemaID: testSchemaID, Version: 2, Checksum: "new-checksum"}, nil)
	store.On("CreateSchemaField", ctx, mock.AnythingOfType("schema.CreateSchemaFieldParams")).
		Return(domain.SchemaField{Path: "app.name", FieldType: "string"}, nil)

	resp, err := svc.ImportSchema(ctx, &pb.ImportSchemaRequest{
		YamlContent: validYAML("my-schema"),
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Schema.Version)
	store.AssertExpectations(t)
}

func TestImportSchema_IdenticalChecksum_AlreadyExists(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	// Compute the real checksum that the service will produce for this YAML.
	yamlContent := validYAML("my-schema")
	doc, err := unmarshalSchemaYAML(yamlContent)
	require.NoError(t, err)
	fields := yamlToProtoFields(doc)
	checksum := computeChecksum(fields)

	existingSchema := domain.Schema{ID: testSchemaID, Name: "my-schema"}
	latestVersion := domain.SchemaVersion{
		ID: testVersionID, SchemaID: testSchemaID,
		Version: 1, Checksum: checksum,
	}

	store.On("GetSchemaByName", ctx, "my-schema").Return(existingSchema, nil)
	store.On("GetLatestSchemaVersion", ctx, testSchemaID).Return(latestVersion, nil)
	store.On("GetSchemaFields", ctx, testVersionID).
		Return([]domain.SchemaField{{Path: "app.name", FieldType: "string"}}, nil)

	resp, err := svc.ImportSchema(ctx, &pb.ImportSchemaRequest{
		YamlContent: yamlContent,
	})
	// Returns both a response and an AlreadyExists error.
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
	require.NotNil(t, resp)
	assert.Equal(t, "my-schema", resp.Schema.Name)
	store.AssertExpectations(t)
}

func TestImportSchema_InvalidYAML(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	_, err := svc.ImportSchema(superadminCtx(), &pb.ImportSchemaRequest{
		YamlContent: []byte("not: valid: yaml: content"),
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestImportSchema_MissingSyntax(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))

	_, err := svc.ImportSchema(superadminCtx(), &pb.ImportSchemaRequest{
		YamlContent: []byte("name: test\nfields:\n  app.name:\n    type: string\n"),
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestImportSchema_LookupError(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	store.On("GetSchemaByName", ctx, "my-schema").
		Return(domain.Schema{}, assert.AnError)

	_, err := svc.ImportSchema(ctx, &pb.ImportSchemaRequest{
		YamlContent: validYAML("my-schema"),
	})
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestImportSchema_ExistingWithAutoPublish(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	existingSchema := domain.Schema{ID: testSchemaID, Name: "my-schema"}
	latestVersion := domain.SchemaVersion{
		ID: "00000000-0000-0000-0000-000000000010", SchemaID: testSchemaID,
		Version: 1, Checksum: "old-checksum",
	}
	newVersionID := "00000000-0000-0000-0000-000000000011"

	store.On("GetSchemaByName", ctx, "my-schema").Return(existingSchema, nil)
	store.On("GetLatestSchemaVersion", ctx, testSchemaID).Return(latestVersion, nil)
	store.On("CreateSchemaVersion", ctx, mock.AnythingOfType("schema.CreateSchemaVersionParams")).
		Return(domain.SchemaVersion{ID: newVersionID, SchemaID: testSchemaID, Version: 2, Checksum: "new"}, nil)
	store.On("CreateSchemaField", ctx, mock.AnythingOfType("schema.CreateSchemaFieldParams")).
		Return(domain.SchemaField{Path: "app.name", FieldType: "string"}, nil)
	// autoPublish flow
	store.On("GetSchemaByID", ctx, testSchemaID).
		Return(existingSchema, nil)
	store.On("PublishSchemaVersion", ctx, PublishSchemaVersionParams{SchemaID: testSchemaID, Version: 2}).
		Return(domain.SchemaVersion{ID: newVersionID, Version: 2, Published: true}, nil)
	store.On("GetSchemaFields", ctx, newVersionID).
		Return([]domain.SchemaField{{Path: "app.name", FieldType: "string"}}, nil)

	resp, err := svc.ImportSchema(ctx, &pb.ImportSchemaRequest{
		YamlContent: validYAML("my-schema"),
		AutoPublish: true,
	})
	require.NoError(t, err)
	assert.True(t, resp.Schema.Published)
	assert.Equal(t, int32(2), resp.Schema.Version)
	store.AssertExpectations(t)
}

func TestImportSchema_BadRegexConstraint_NewSchema(t *testing.T) {
	// A schema with an invalid regex pattern must be rejected at import time
	// (InvalidArgument) before any storage write, not silently persisted and
	// only caught at validation time with a confusing error far from the root cause.
	//
	// Patterns are double-quoted in the YAML to avoid YAML-level parse errors;
	// the invalidity must be caught at the Go regexp compilation stage.
	badPatterns := []struct{ name, pat string }{
		{"unclosed_bracket", `"(?P<name"`},
		{"bare_repetition", `"(?i"`},
	}
	for _, tc := range badPatterns {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockStore{}
			svc := NewService(store, WithLogger(testLogger))

			yaml := []byte("spec_version: v1\nname: bad-regex\nfields:\n  app.name:\n    type: string\n    constraints:\n      pattern: " + tc.pat + "\n")
			_, err := svc.ImportSchema(superadminCtx(), &pb.ImportSchemaRequest{
				YamlContent: yaml,
			})
			assert.Equal(t, codes.InvalidArgument, status.Code(err), "expected InvalidArgument for pattern %s", tc.pat)
			assert.Contains(t, err.Error(), "invalid regex constraint", "expected error message for pattern %s", tc.pat)
			// No storage calls should have been made.
			store.AssertNotCalled(t, "GetSchemaByName")
			store.AssertNotCalled(t, "CreateSchema")
		})
	}
}

func TestImportSchema_BadRegexConstraint_ExistingSchema(t *testing.T) {
	// Importing a new version of an existing schema with a bad regex must also
	// be rejected before any storage write occurs.
	store := &mockStore{}
	svc := NewService(store, WithLogger(testLogger))
	ctx := superadminCtx()

	// The constraint check runs before the schema name lookup, so no store
	// calls are expected at all.
	yaml := []byte("spec_version: v1\nname: my-schema\nfields:\n  app.name:\n    type: string\n    constraints:\n      pattern: \"(?P<name\"\n")
	_, err := svc.ImportSchema(ctx, &pb.ImportSchemaRequest{
		YamlContent: yaml,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "invalid regex constraint")
	// Verify no storage calls were made — the error must surface before any write.
	store.AssertNotCalled(t, "GetSchemaByName")
	store.AssertNotCalled(t, "CreateSchema")
	store.AssertNotCalled(t, "CreateSchemaVersion")
}
