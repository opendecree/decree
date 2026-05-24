package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	assert.Equal(t, 1_000, l.MaxListLen)
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
