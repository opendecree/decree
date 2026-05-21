package grpctransport

import (
	"crypto/tls"
	"crypto/x509"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// DialOption configures [Dial].
type DialOption func(*dialConfig)

type dialConfig struct {
	insecure bool
	customCA *x509.CertPool
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

// Dial opens a gRPC connection to target with TLS and system roots by default.
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
	var cfg dialConfig
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
		creds = credentials.NewTLS(&tls.Config{})
	}

	return grpc.NewClient(target, grpc.WithTransportCredentials(creds))
}
