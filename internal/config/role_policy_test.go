package config

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

// userCtx returns a context with user-role claims scoped to tenantID1.
func userCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleUser,
		TenantIDs: []string{tenantID1},
	})
}

func assertPermissionDenied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// --- user role must be denied on all write RPCs ---

func TestSetField_DeniedForUser(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.SetField(userCtx(), &pb.SetFieldRequest{TenantId: tenantID1, FieldPath: "app.x"})
	assertPermissionDenied(t, err)
}

func TestSetFields_DeniedForUser(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.SetFields(userCtx(), &pb.SetFieldsRequest{TenantId: tenantID1})
	assertPermissionDenied(t, err)
}

func TestRollbackToVersion_DeniedForUser(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.RollbackToVersion(userCtx(), &pb.RollbackToVersionRequest{TenantId: tenantID1, Version: 1})
	assertPermissionDenied(t, err)
}

func TestImportConfig_DeniedForUser(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.ImportConfig(userCtx(), &pb.ImportConfigRequest{TenantId: tenantID1})
	assertPermissionDenied(t, err)
}
