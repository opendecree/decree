package grpctransport

import (
	"google.golang.org/grpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/configwatcher"
)

// NewConfigClient creates a [configclient.Client] backed by a gRPC connection.
//
// Options configure both the transport (auth) and the client (retry).
//
//	conn, _ := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
//	client := grpctransport.NewConfigClient(conn, grpctransport.WithSubject("myapp"))
func NewConfigClient(conn grpc.ClientConnInterface, opts ...Option) *configclient.Client {
	cfg := buildConfig(opts)
	transport := &ConfigTransport{
		rpc:  pb.NewConfigServiceClient(conn),
		auth: cfg.auth,
	}
	return configclient.New(transport, cfg.clientOpts...)
}

// NewAdminClient creates an [adminclient.Client] backed by a gRPC connection.
//
// Options configure the transport auth. All three services (schema, config, audit)
// are configured using the same connection.
//
//	conn, _ := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
//	client := grpctransport.NewAdminClient(conn, grpctransport.WithSubject("admin"))
func NewAdminClient(conn grpc.ClientConnInterface, opts ...Option) *adminclient.Client {
	cfg := buildConfig(opts)
	return adminclient.New(
		&SchemaTransport{rpc: pb.NewSchemaServiceClient(conn), auth: cfg.auth},
		&AdminConfigTransport{rpc: pb.NewConfigServiceClient(conn), auth: cfg.auth},
		&AuditTransport{rpc: pb.NewAuditServiceClient(conn), auth: cfg.auth},
	)
}

// NewWatcher creates a [configwatcher.Watcher] backed by a gRPC connection.
//
// Options configure both the transport (auth) and the watcher (reconnect backoff, logger).
//
//	conn, _ := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
//	w := grpctransport.NewWatcher(conn, "tenant-uuid", grpctransport.WithSubject("myapp"))
func NewWatcher(conn grpc.ClientConnInterface, tenantID string, opts ...Option) *configwatcher.Watcher {
	cfg := buildConfig(opts)
	transport := &ConfigTransport{
		rpc:  pb.NewConfigServiceClient(conn),
		auth: cfg.auth,
	}
	return configwatcher.New(transport, tenantID, cfg.watcherOpts...)
}
