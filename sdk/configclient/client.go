package configclient

import (
	"context"

	"google.golang.org/grpc/metadata"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// Client wraps the generated ConfigService gRPC client with an ergonomic API
// for reading and writing configuration values.
//
// All methods are safe for concurrent use.
type Client struct {
	rpc  pb.ConfigServiceClient
	opts options
}

// New creates a new config client wrapping the given gRPC client.
// Options configure default authentication metadata injected into every call.
//
// Example:
//
//	conn, _ := grpc.Dial("localhost:9090", grpc.WithInsecure())
//	rpc := pb.NewConfigServiceClient(conn)
//	client := configclient.New(rpc, configclient.WithSubject("myapp"))
func New(client pb.ConfigServiceClient, opts ...Option) *Client {
	o := options{role: "superadmin"}
	for _, opt := range opts {
		opt(&o)
	}
	return &Client{rpc: client, opts: o}
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
// This identifies the actor making the request.
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
// This is an alternative to metadata-based auth (WithSubject/WithRole/WithTenantID).
func WithBearerToken(token string) Option {
	return func(o *options) { o.bearerToken = token }
}

// withAuth returns a context with the configured auth metadata.
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
