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

// New creates a new admin client. Pass [WithSchemaTransport],
// [WithConfigTransport], [WithAuditTransport], and [WithServerTransport] for
// the services you need. Methods on services without a wired transport return
// [ErrServiceNotConfigured].
//
// Example (with grpctransport):
//
//	client := grpctransport.NewAdminClient(conn, grpctransport.WithSubject("admin"))
//
// Example (with explicit transports):
//
//	client := adminclient.New(
//		adminclient.WithSchemaTransport(schemaTransport),
//		adminclient.WithConfigTransport(configTransport),
//		adminclient.WithAuditTransport(auditTransport),
//		adminclient.WithServerTransport(serverTransport),
//	)
func New(opts ...Option) *Client {
	o := clientOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	return &Client{schema: o.schema, config: o.config, audit: o.audit, server: o.server}
}
