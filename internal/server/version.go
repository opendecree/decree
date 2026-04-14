package server

import (
	"context"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/version"
)

// VersionService implements the VersionService gRPC server.
type VersionService struct {
	pb.UnimplementedVersionServiceServer
}

// GetServerVersion returns the server's build version and commit hash.
func (s *VersionService) GetServerVersion(_ context.Context, _ *pb.GetServerVersionRequest) (*pb.GetServerVersionResponse, error) {
	return &pb.GetServerVersionResponse{
		Version: version.Version,
		Commit:  version.Commit,
	}, nil
}
