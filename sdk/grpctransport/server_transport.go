package grpctransport

import (
	"context"

	"google.golang.org/grpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
)

// ServerTransport implements [adminclient.ServerTransport] using gRPC.
type ServerTransport struct {
	rpc pb.ServerServiceClient
}

// Compile-time check.
var _ adminclient.ServerTransport = (*ServerTransport)(nil)

// NewServerTransport creates a new gRPC-backed server info transport.
func NewServerTransport(conn grpc.ClientConnInterface) *ServerTransport {
	return &ServerTransport{rpc: pb.NewServerServiceClient(conn)}
}

func (t *ServerTransport) GetServerInfo(ctx context.Context) (*adminclient.ServerInfo, error) {
	resp, err := t.rpc.GetServerInfo(ctx, &pb.GetServerInfoRequest{})
	if err != nil {
		return nil, err
	}
	return &adminclient.ServerInfo{
		Version:  resp.Version,
		Commit:   resp.Commit,
		Features: resp.Features,
	}, nil
}
