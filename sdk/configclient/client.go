// Package configclient provides an ergonomic Go client for reading and
// writing OpenDecree configuration values at application runtime.
//
// The client wraps a pluggable [Transport] (see the sibling grpctransport
// module for the gRPC implementation) with typed accessors and optimistic
// concurrency helpers. For administrative operations such as schema and
// tenant management, see the adminclient package. For a live, auto-refreshing
// view over configuration, see configwatcher.
package configclient

// Client wraps a [Transport] with an ergonomic API for reading and writing
// configuration values.
//
// All methods are safe for concurrent use.
type Client struct {
	transport Transport
	opts      options
}

// New creates a new config client using the given transport.
// Options configure client behavior such as automatic retry.
//
// Example (with grpctransport):
//
//	transport := grpctransport.NewConfigTransport(conn, grpctransport.WithSubject("myapp"))
//	client := configclient.New(transport)
//
// Example (with convenience helper):
//
//	client := grpctransport.NewConfigClient(conn, grpctransport.WithSubject("myapp"))
func New(transport Transport, opts ...Option) *Client {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return &Client{transport: transport, opts: o}
}

// Option configures the client's behavior.
type Option func(*options)

type options struct {
	retryEnabled bool
	retry        RetryConfig
}
