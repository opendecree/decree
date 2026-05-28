package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/authz"
	"github.com/opendecree/decree/internal/storage/domain"
)

// stubGuard is a Guard that always returns a fixed error (or nil).
type stubGuard struct{ err error }

func (g stubGuard) Check(_ context.Context, _ authz.Action, _ authz.Resource) error {
	return g.err
}

func TestNewService_WithGuard_ReplacesDefaultGuard(t *testing.T) {
	store := &mockStore{}
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}

	guardErr := status.Error(codes.PermissionDenied, "blocked by test guard")
	svc := NewService(store, c, pub, sub,
		WithLogger(testLogger),
		WithGuard(stubGuard{err: guardErr}),
	)

	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{ID: tenantID1}, nil)

	_, err := svc.GetConfig(auth.WithoutAuth(context.Background()), &pb.GetConfigRequest{TenantId: tenantID1})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestNewService_WithGuard_AllowAll(t *testing.T) {
	store := &mockStore{}
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}

	svc := NewService(store, c, pub, sub,
		WithLogger(testLogger),
		WithGuard(stubGuard{err: nil}),
	)

	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{ID: tenantID1}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	c.On("Get", mock.Anything, tenantID1, int32(1), int32(0)).
		Return(map[string]string{}, nil)
	setupNoSensitiveFields(store)

	_, err := svc.GetConfig(auth.WithoutAuth(context.Background()), &pb.GetConfigRequest{TenantId: tenantID1})
	require.NoError(t, err)
}
