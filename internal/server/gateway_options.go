package server

import "log/slog"

// GatewayOption configures a Gateway.
type GatewayOption func(*gatewayOptions)

type gatewayOptions struct {
	logger          *slog.Logger
	openAPISpec     []byte
	maxRecvMsgBytes int
	maxSendMsgBytes int
	tls             *GatewayTLSConfig
	insecure        bool
}

// WithGatewayLogger sets the gateway logger. Defaults to slog.Default() when unset.
func WithGatewayLogger(l *slog.Logger) GatewayOption {
	return func(o *gatewayOptions) { o.logger = l }
}

// WithOpenAPISpec serves the given raw OpenAPI JSON at /docs/openapi.json
// (and the Swagger UI at /docs). When unset, the docs endpoints are not
// registered.
func WithOpenAPISpec(spec []byte) GatewayOption {
	return func(o *gatewayOptions) { o.openAPISpec = spec }
}

// WithGatewayMaxRecvMsgBytes caps inbound gRPC response size from the upstream
// server. Zero or negative falls back to DefaultMaxMsgBytes.
func WithGatewayMaxRecvMsgBytes(n int) GatewayOption {
	return func(o *gatewayOptions) { o.maxRecvMsgBytes = n }
}

// WithGatewayMaxSendMsgBytes caps outbound gRPC request size to the upstream
// server. Zero or negative falls back to DefaultMaxMsgBytes.
func WithGatewayMaxSendMsgBytes(n int) GatewayOption {
	return func(o *gatewayOptions) { o.maxSendMsgBytes = n }
}

// WithGatewayTLS configures the upstream gRPC dial. Required unless
// WithGatewayInsecure is set; mutually exclusive with WithGatewayInsecure.
func WithGatewayTLS(cfg *GatewayTLSConfig) GatewayOption {
	return func(o *gatewayOptions) { o.tls = cfg }
}

// WithGatewayInsecure dials the upstream gRPC server in plaintext
// (INSECURE_LISTEN=1). Mutually exclusive with WithGatewayTLS.
func WithGatewayInsecure() GatewayOption {
	return func(o *gatewayOptions) { o.insecure = true }
}
