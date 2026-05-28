package server

import (
	"io/fs"
	"log/slog"
)

// GatewayOption configures a Gateway.
type GatewayOption func(*gatewayOptions)

type gatewayOptions struct {
	logger              *slog.Logger
	openAPISpec         []byte
	uiFS                fs.FS
	maxRecvMsgBytes     int
	maxSendMsgBytes     int
	tls                 *GatewayTLSConfig
	serverTLS           *TLSConfig
	insecure            bool
	trustedProxy        bool
	corsOrigins         []string
	docsProtected       bool
	plaintextTerminator bool
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

// WithGatewayCORSOrigins registers an explicit allowlist of Origins that the
// gateway will echo back in Access-Control-Allow-Origin responses. Only listed
// origins receive CORS headers; wildcard * is never used so that
// Access-Control-Allow-Credentials can safely be set. An empty list disables
// CORS headers entirely.
func WithGatewayCORSOrigins(origins []string) GatewayOption {
	return func(o *gatewayOptions) { o.corsOrigins = origins }
}

// WithGatewayDocsProtected gates the /docs and /docs/openapi.json endpoints
// behind a simple presence check: requests without an Authorization header
// receive 401. Combine with WithGatewayTrustedProxy when the proxy injects the
// credential.
func WithGatewayDocsProtected() GatewayOption {
	return func(o *gatewayOptions) { o.docsProtected = true }
}

// WithGatewayTrustedProxy declares that a trusted authentication proxy sits in
// front of the gateway and is allowed to set x-subject, x-role, and
// x-tenant-id headers. Without this option (the default), the gateway rejects
// any request that carries those headers to prevent client impersonation.
// Set DECREE_GATEWAY_TRUSTED_PROXY=1 to enable at runtime.
func WithGatewayTrustedProxy() GatewayOption {
	return func(o *gatewayOptions) { o.trustedProxy = true }
}

// WithGatewayServerTLS enables HTTPS on the gateway's inbound HTTP listener.
// The gateway serves TLS-terminated HTTPS to clients, independent of the
// outbound gRPC dial configuration (WithGatewayTLS / WithGatewayInsecure).
// Certificates are reloaded from disk on every TLS handshake.
func WithGatewayServerTLS(cfg *TLSConfig) GatewayOption {
	return func(o *gatewayOptions) { o.serverTLS = cfg }
}

// WithGatewayPlaintextTerminator acknowledges that a trusted TLS-terminating
// proxy sits in front of the gateway so that plaintext HTTP on a non-loopback
// address is intentional. Without this option, NewGateway rejects plaintext
// binds on all-interface addresses to prevent accidental public exposure.
func WithGatewayPlaintextTerminator() GatewayOption {
	return func(o *gatewayOptions) { o.plaintextTerminator = true }
}
