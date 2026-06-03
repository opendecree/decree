package grpctransport

import (
	"google.golang.org/grpc"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/configwatcher"
)

// NewConfigClient creates a [configclient.Client] backed by a gRPC connection.
//
// Options configure both the transport (auth) and the client (retry).
// WithRole (or WithBearerToken) is required; construction returns an error if omitted.
//
//	conn, _ := grpctransport.Dial("localhost:50051", grpctransport.WithInsecure())
//	client, err := grpctransport.NewConfigClient(conn, grpctransport.WithSubject("myapp"), grpctransport.WithRole("user"))
func NewConfigClient(conn grpc.ClientConnInterface, opts ...Option) (*configclient.Client, error) {
	cfg, err := buildConfig(opts)
	if err != nil {
		return nil, err
	}
	transport, err := NewConfigTransport(conn, opts...)
	if err != nil {
		return nil, err
	}
	return configclient.New(transport, cfg.clientOpts...), nil
}

// NewAdminClient creates an [adminclient.Client] backed by a gRPC connection.
//
// Options configure the transport auth. All three services (schema, config, audit)
// are configured using the same connection.
// WithRole (or WithBearerToken) is required; construction returns an error if omitted.
//
//	conn, _ := grpctransport.Dial("localhost:50051", grpctransport.WithInsecure())
//	client, err := grpctransport.NewAdminClient(conn, grpctransport.WithSubject("admin"), grpctransport.WithRole("superadmin"))
func NewAdminClient(conn grpc.ClientConnInterface, opts ...Option) (*adminclient.Client, error) {
	schemaTr, err := NewSchemaTransport(conn, opts...)
	if err != nil {
		return nil, err
	}
	configTr, err := NewAdminConfigTransport(conn, opts...)
	if err != nil {
		return nil, err
	}
	auditTr, err := NewAuditTransport(conn, opts...)
	if err != nil {
		return nil, err
	}
	return adminclient.New(
		adminclient.WithSchemaTransport(schemaTr),
		adminclient.WithConfigTransport(configTr),
		adminclient.WithAuditTransport(auditTr),
		adminclient.WithServerTransport(NewServerTransport(conn)),
	), nil
}

// NewWatcher creates a [configwatcher.Watcher] backed by a gRPC connection.
//
// Options configure both the transport (auth) and the watcher (reconnect backoff, logger).
// WithRole (or WithBearerToken) is required; construction returns an error if omitted.
//
//	conn, _ := grpctransport.Dial("localhost:50051", grpctransport.WithInsecure())
//	w, err := grpctransport.NewWatcher(conn, "tenant-uuid", grpctransport.WithSubject("myapp"), grpctransport.WithRole("user"))
func NewWatcher(conn grpc.ClientConnInterface, tenantID string, opts ...Option) (*configwatcher.Watcher, error) {
	cfg, err := buildConfig(opts)
	if err != nil {
		return nil, err
	}
	transport, err := NewConfigTransport(conn, opts...)
	if err != nil {
		return nil, err
	}
	return configwatcher.New(transport, tenantID, cfg.watcherOpts...), nil
}
