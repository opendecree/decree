// Package adminclient provides an ergonomic Go client for administrative
// operations on the Central Config Service: schema management, tenant management,
// field locks, audit queries, and config versioning/import/export.
//
// For application-runtime config reads and writes, see the configclient package.
package adminclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// Client wraps the generated SchemaService, ConfigService, and AuditService
// gRPC clients with an ergonomic API for administrative operations.
//
// All methods are safe for concurrent use.
type Client struct {
	schema pb.SchemaServiceClient
	config pb.ConfigServiceClient
	audit  pb.AuditServiceClient
	opts   options
}

// New creates a new admin client wrapping the given gRPC service clients.
// Any of the service clients may be nil if that service is not needed;
// methods for a nil service will return [ErrServiceNotConfigured].
//
// Example:
//
//	conn, _ := grpc.Dial("localhost:9090", grpc.WithInsecure())
//	client := adminclient.New(
//	    pb.NewSchemaServiceClient(conn),
//	    pb.NewConfigServiceClient(conn),
//	    pb.NewAuditServiceClient(conn),
//	    adminclient.WithSubject("admin@example.com"),
//	)
func New(schema pb.SchemaServiceClient, config pb.ConfigServiceClient, audit pb.AuditServiceClient, opts ...Option) *Client {
	o := options{role: "superadmin"}
	for _, opt := range opts {
		opt(&o)
	}
	return &Client{schema: schema, config: config, audit: audit, opts: o}
}

// Option configures the client's default behavior.
type Option func(*options)

type options struct {
	subject     string
	role        string
	tenantID    string
	bearerToken string
}

// WithSubject sets the x-subject metadata header on every call.
func WithSubject(subject string) Option {
	return func(o *options) { o.subject = subject }
}

// WithRole sets the x-role metadata header on every call.
// Valid values are "superadmin", "admin", and "user". Defaults to "superadmin".
func WithRole(role string) Option {
	return func(o *options) { o.role = role }
}

// WithTenantID sets the x-tenant-id metadata header on every call.
// Required for non-superadmin roles.
func WithTenantID(tenantID string) Option {
	return func(o *options) { o.tenantID = tenantID }
}

// WithBearerToken sets a JWT bearer token on every call.
// This is an alternative to metadata-based auth.
func WithBearerToken(token string) Option {
	return func(o *options) { o.bearerToken = token }
}

func (c *Client) withAuth(ctx context.Context) context.Context {
	if c.opts.bearerToken != "" {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.opts.bearerToken)
	}
	pairs := make([]string, 0, 6)
	if c.opts.subject != "" {
		pairs = append(pairs, "x-subject", c.opts.subject)
	}
	if c.opts.role != "" {
		pairs = append(pairs, "x-role", c.opts.role)
	}
	if c.opts.tenantID != "" {
		pairs = append(pairs, "x-tenant-id", c.opts.tenantID)
	}
	if len(pairs) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}
