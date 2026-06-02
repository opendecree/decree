package adminclient

// Option configures a Client.
type Option func(*clientOptions)

type clientOptions struct {
	schema       SchemaTransport
	config       ConfigTransport
	audit        AuditTransport
	server       ServerTransport
	retryEnabled bool
	retry        RetryConfig
}

// WithSchemaTransport wires the schema transport. Methods that need it return
// [ErrServiceNotConfigured] when unset.
func WithSchemaTransport(t SchemaTransport) Option {
	return func(o *clientOptions) { o.schema = t }
}

// WithConfigTransport wires the config transport. Methods that need it return
// [ErrServiceNotConfigured] when unset.
func WithConfigTransport(t ConfigTransport) Option {
	return func(o *clientOptions) { o.config = t }
}

// WithAuditTransport wires the audit transport. Methods that need it return
// [ErrServiceNotConfigured] when unset.
func WithAuditTransport(t AuditTransport) Option {
	return func(o *clientOptions) { o.audit = t }
}

// WithServerTransport wires the server-info transport. Methods that need it
// return [ErrServiceNotConfigured] when unset.
func WithServerTransport(t ServerTransport) Option {
	return func(o *clientOptions) { o.server = t }
}

// WithRetry enables automatic retry with exponential backoff for transient errors.
// Only idempotent read operations (List*, Get*) are retried by default.
// By default, only errors wrapped in [RetryableError] by the transport are retried.
func WithRetry(cfg RetryConfig) Option {
	return func(o *clientOptions) {
		o.retry = cfg.WithDefaults()
		o.retryEnabled = true
	}
}
