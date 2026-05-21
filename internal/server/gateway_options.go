package server

import (
	"io/fs"
	"log/slog"
)

// GatewayOption configures a Gateway.
type GatewayOption func(*gatewayOptions)

type gatewayOptions struct {
	logger          *slog.Logger
	openAPISpec     []byte
	uiFS            fs.FS
	maxRecvMsgBytes int
	maxSendMsgBytes int
	tls             *GatewayTLSConfig
	insecure        bool
	trustedProxy    bool
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

// WithUI serves the given filesystem at /admin/ with SPA client-side routing
// fallback. The FS must be rooted at the dist directory (index.html at its
// root). When unset, the /admin/ route is not registered.
func WithUI(fsys fs.FS) GatewayOption {
	return func(o *gatewayOptions) { o.uiFS = fsys }
}

// WithGatewayTrustedProxy declares that a trusted authentication proxy sits in
// front of the gateway and is allowed to set x-subject, x-role, and
// x-tenant-id headers. Without this option (the default), the gateway rejects
// any request that carries those headers to prevent client impersonation.
// Set DECREE_GATEWAY_TRUSTED_PROXY=1 to enable at runtime.
func WithGatewayTrustedProxy() GatewayOption {
	return func(o *gatewayOptions) { o.trustedProxy = true }
}
