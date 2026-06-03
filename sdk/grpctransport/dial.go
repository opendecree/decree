package grpctransport

import (
	"crypto/tls"
	"crypto/x509"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// defaultKeepalive is applied to every Dial call unless overridden via
// [WithKeepalive]. These values prevent silent connection drops on long-lived
// watch streams behind NAT/firewalls:
//   - Time=30s: send a keepalive ping after 30s of inactivity.
//   - Timeout=10s: treat the connection as dead if no ACK arrives within 10s.
//   - PermitWithoutStream=true: send pings even when no RPCs are in flight
//     (critical for configwatcher, which holds an idle subscription stream).
var defaultKeepalive = keepalive.ClientParameters{
	Time:                30 * time.Second,
	Timeout:             10 * time.Second,
	PermitWithoutStream: true,
}

// DefaultKeepalive returns the keepalive parameters applied by [Dial] when
// [WithKeepalive] is not provided. Callers can use this to inspect or modify
// the defaults before passing them to [WithKeepalive].
func DefaultKeepalive() keepalive.ClientParameters { return defaultKeepalive }

// DialOption configures [Dial].
type DialOption func(*dialConfig)

type dialConfig struct {
	insecure  bool
	customCA  *x509.CertPool
	keepalive keepalive.ClientParameters
}

// WithCustomCA configures TLS with the provided certificate pool instead of system roots.
func WithCustomCA(pool *x509.CertPool) DialOption {
	return func(c *dialConfig) { c.customCA = pool }
}

// WithInsecure disables TLS entirely. Only use for local development or testing.
// Production connections must use TLS.
func WithInsecure() DialOption {
	return func(c *dialConfig) { c.insecure = true }
}

// WithKeepalive overrides the default keepalive parameters applied by [Dial].
//
// The defaults (Time=30s, Timeout=10s, PermitWithoutStream=true) are chosen
// for long-lived watch streams. Override only when you have specific network
// requirements — for example, a low-latency private network that tolerates
// longer ping intervals.
func WithKeepalive(params keepalive.ClientParameters) DialOption {
	return func(c *dialConfig) { c.keepalive = params }
}

// Dial opens a gRPC connection to target with TLS and system roots by default.
//
// The caller owns the returned [*grpc.ClientConn] and must call Close on it
// when done, even if the connection was also passed to a transport constructor.
//
// Keepalive is enabled by default (Time=30s, Timeout=10s,
// PermitWithoutStream=true) to prevent silent connection drops on long-lived
// watch streams. Override with [WithKeepalive].
//
// For production use, omit options to get TLS with system certificate roots:
//
//	conn, err := grpctransport.Dial("api.example.com:443")
//
// For local development against an unencrypted server:
//
//	conn, err := grpctransport.Dial("localhost:9090", grpctransport.WithInsecure())
//
// For a private CA (internal services, mTLS):
//
//	conn, err := grpctransport.Dial("internal:9090", grpctransport.WithCustomCA(pool))
func Dial(target string, opts ...DialOption) (*grpc.ClientConn, error) {
	cfg := dialConfig{keepalive: defaultKeepalive}
	for _, o := range opts {
		o(&cfg)
	}

	var creds credentials.TransportCredentials
	switch {
	case cfg.insecure:
		creds = insecure.NewCredentials()
	case cfg.customCA != nil:
		creds = credentials.NewTLS(&tls.Config{RootCAs: cfg.customCA})
	default:
		// An empty tls.Config uses system roots, derives the SNI from the target
		// address, and enables full certificate verification. This is the secure
		// default for production connections.
		creds = credentials.NewTLS(&tls.Config{})
	}

	return grpc.NewClient(target,
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(cfg.keepalive),
	)
}
