package grpctransport

import (
	"google.golang.org/grpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/configclient"
)

// grpcSubscription wraps a gRPC server-streaming client as a configclient.Subscription.
type grpcSubscription struct {
	stream grpc.ServerStreamingClient[pb.SubscribeResponse]
}

// Compile-time check.
var _ configclient.Subscription = (*grpcSubscription)(nil)

func (s *grpcSubscription) Recv() (*configclient.ConfigChange, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return nil, mapConfigError(err)
	}
	change := resp.GetChange()
	if change == nil {
		return &configclient.ConfigChange{}, nil
	}
	return &configclient.ConfigChange{
		TenantID:  change.GetTenantId(),
		FieldPath: change.GetFieldPath(),
		OldValue:  typedValueFromProto(change.GetOldValue()),
		NewValue:  typedValueFromProto(change.GetNewValue()),
	}, nil
}
