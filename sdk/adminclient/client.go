// Package adminclient provides an ergonomic Go client for administrative
// operations on OpenDecree: schema management, tenant management,
// field locks, audit queries, and config versioning/import/export.
//
// For application-runtime config reads and writes, see the configclient package.
package adminclient

// Client wraps transport implementations for SchemaService, ConfigService,
// AuditService, and ServerService with an ergonomic API for administrative operations.
//
// All methods are safe for concurrent use.
type Client struct {
	schema SchemaTransport
	config ConfigTransport
	audit  AuditTransport
	server ServerTransport
}

// New creates a new admin client using the given transport implementations.
// Any of the transports may be nil if that service is not needed;
// methods for a nil service will return [ErrServiceNotConfigured].
//
// Example (with grpctransport):
//
//	client := grpctransport.NewAdminClient(conn, grpctransport.WithSubject("admin"))
//
// Example (with explicit transports):
//
//	client := adminclient.New(schemaTransport, configTransport, auditTransport, serverTransport)
func New(schema SchemaTransport, config ConfigTransport, audit AuditTransport, server ServerTransport) *Client {
	return &Client{schema: schema, config: config, audit: audit, server: server}
}
