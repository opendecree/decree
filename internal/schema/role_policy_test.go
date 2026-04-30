package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
)

// userCtx returns a context carrying user-role claims scoped to testTenantID.
func userCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleUser,
		TenantIDs: []string{testTenantID},
	})
}

// adminCtx returns a context carrying admin-role claims scoped to testTenantID.
func adminCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{testTenantID},
	})
}

func assertPermissionDenied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// --- superadmin-only RPCs: admin and user must be denied ---

func TestCreateSchema_DeniedForAdmin(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.CreateSchema(adminCtx(), &pb.CreateSchemaRequest{Name: "s"})
	assertPermissionDenied(t, err)
}

func TestCreateSchema_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.CreateSchema(userCtx(), &pb.CreateSchemaRequest{Name: "s"})
	assertPermissionDenied(t, err)
}

func TestUpdateSchema_DeniedForAdmin(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.UpdateSchema(adminCtx(), &pb.UpdateSchemaRequest{Id: testSchemaID})
	assertPermissionDenied(t, err)
}

func TestUpdateSchema_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.UpdateSchema(userCtx(), &pb.UpdateSchemaRequest{Id: testSchemaID})
	assertPermissionDenied(t, err)
}

func TestDeleteSchema_DeniedForAdmin(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.DeleteSchema(adminCtx(), &pb.DeleteSchemaRequest{Id: testSchemaID})
	assertPermissionDenied(t, err)
}

func TestDeleteSchema_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.DeleteSchema(userCtx(), &pb.DeleteSchemaRequest{Id: testSchemaID})
	assertPermissionDenied(t, err)
}

func TestPublishSchema_DeniedForAdmin(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.PublishSchema(adminCtx(), &pb.PublishSchemaRequest{Id: testSchemaID})
	assertPermissionDenied(t, err)
}

func TestPublishSchema_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.PublishSchema(userCtx(), &pb.PublishSchemaRequest{Id: testSchemaID})
	assertPermissionDenied(t, err)
}

func TestImportSchema_DeniedForAdmin(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.ImportSchema(adminCtx(), &pb.ImportSchemaRequest{YamlContent: []byte("x: 1")})
	assertPermissionDenied(t, err)
}

func TestImportSchema_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.ImportSchema(userCtx(), &pb.ImportSchemaRequest{YamlContent: []byte("x: 1")})
	assertPermissionDenied(t, err)
}

func TestCreateTenant_DeniedForAdmin(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.CreateTenant(adminCtx(), &pb.CreateTenantRequest{
		Name:     "t",
		SchemaId: testSchemaID,
	})
	assertPermissionDenied(t, err)
}

func TestCreateTenant_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.CreateTenant(userCtx(), &pb.CreateTenantRequest{
		Name:     "t",
		SchemaId: testSchemaID,
	})
	assertPermissionDenied(t, err)
}

// --- admin-or-above RPCs: user must be denied, admin must pass role check ---

func TestUpdateTenant_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.UpdateTenant(userCtx(), &pb.UpdateTenantRequest{Id: testTenantID})
	assertPermissionDenied(t, err)
}

func TestDeleteTenant_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.DeleteTenant(userCtx(), &pb.DeleteTenantRequest{Id: testTenantID})
	assertPermissionDenied(t, err)
}

func TestLockField_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.LockField(userCtx(), &pb.LockFieldRequest{
		TenantId:  testTenantID,
		FieldPath: "app.x",
	})
	assertPermissionDenied(t, err)
}

func TestUnlockField_DeniedForUser(t *testing.T) {
	svc := NewService(&mockStore{}, WithLogger(testLogger))
	_, err := svc.UnlockField(userCtx(), &pb.UnlockFieldRequest{
		TenantId:  testTenantID,
		FieldPath: "app.x",
	})
	assertPermissionDenied(t, err)
}
