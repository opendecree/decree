package server

import (
	"context"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/version"
)

// Features holds the enabled state of server features.
type Features struct {
	Schema        bool
	Config        bool
	Audit         bool
	UsageTracking bool
	JWTAuth       bool
	HTTPGateway   bool
}

// ServerService implements the ServerService gRPC server.
type ServerService struct {
	pb.UnimplementedServerServiceServer
	features Features
}

// NewServerService creates a new ServerService with the given feature flags.
func NewServerService(features Features) *ServerService {
	return &ServerService{features: features}
}

// GetServerInfo returns the server's version, commit, and enabled features.
func (s *ServerService) GetServerInfo(_ context.Context, _ *pb.GetServerInfoRequest) (*pb.GetServerInfoResponse, error) {
	return &pb.GetServerInfoResponse{
		Version: version.Version,
		Commit:  version.Commit,
		Features: map[string]bool{
			"schema":         s.features.Schema,
			"config":         s.features.Config,
			"audit":          s.features.Audit,
			"usage_tracking": s.features.UsageTracking,
			"jwt_auth":       s.features.JWTAuth,
			"http_gateway":   s.features.HTTPGateway,
		},
	}, nil
}
