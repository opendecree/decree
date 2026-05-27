package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc/credentials"
)

// TLSConfig configures TLS for the gRPC server.
//
// CertFile and KeyFile are required. When ClientCAFile is set, the server
// requires and verifies client certificates (mTLS).
type TLSConfig struct {
	CertFile     string
	KeyFile      string
	ClientCAFile string
}

// ServerCredentials builds gRPC server credentials from the TLS config.
// The minimum TLS version defaults to 1.3; set DECREE_TLS_MIN_VERSION=TLS12
// to allow TLS 1.2 clients (enables an explicit cipher suite allowlist).
// Certificates are reloaded from disk on every TLS handshake so cert rotation
// requires no server restart.
func (c TLSConfig) ServerCredentials() (credentials.TransportCredentials, error) {
	if c.CertFile == "" || c.KeyFile == "" {
		return nil, errors.New("TLS cert and key files are required")
	}
	// Fail fast at startup if the files are unreadable.
	if _, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile); err != nil {
		return nil, fmt.Errorf("load server keypair: %w", err)
	}
	minVer := tlsMinVersion()
	tlsCfg := &tls.Config{
		GetCertificate: certLoader(c.CertFile, c.KeyFile),
		MinVersion:     minVer,
	}
	if minVer == tls.VersionTLS12 {
		tlsCfg.CipherSuites = tls12CipherSuites
	}
	if c.ClientCAFile != "" {
		pool, err := loadCAPool(c.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("load client CA: %w", err)
		}
		tlsCfg.ClientCAs = pool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return credentials.NewTLS(tlsCfg), nil
}

// GatewayTLSConfig configures TLS for the gateway's outbound gRPC dial.
//
// CAFile is the trusted CA for the gRPC server's certificate; if empty, the
// system root pool is used. ServerName overrides the SNI/verification name
// (defaults to the dial address host). ClientCertFile/ClientKeyFile enable
// client certificate auth (mTLS) when the upstream server requires it.
type GatewayTLSConfig struct {
	CAFile         string
	ServerName     string
	ClientCertFile string
	ClientKeyFile  string
}

// ClientCredentials builds gRPC client credentials for the gateway dial.
func (c GatewayTLSConfig) ClientCredentials() (credentials.TransportCredentials, error) {
	minVer := tlsMinVersion()
	tlsCfg := &tls.Config{
		MinVersion: minVer,
	}
	if minVer == tls.VersionTLS12 {
		tlsCfg.CipherSuites = tls12CipherSuites
	}
	if c.CAFile != "" {
		pool, err := loadCAPool(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("load gateway CA: %w", err)
		}
		tlsCfg.RootCAs = pool
	}
	if c.ServerName != "" {
		tlsCfg.ServerName = c.ServerName
	}
	if c.ClientCertFile != "" || c.ClientKeyFile != "" {
		if c.ClientCertFile == "" || c.ClientKeyFile == "" {
			return nil, errors.New("gateway client cert and key must be set together")
		}
		cert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load gateway client keypair: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(tlsCfg), nil
}

// tlsMinVersion returns the configured TLS floor version.
// DECREE_TLS_MIN_VERSION=TLS12 enables TLS 1.2 for legacy clients.
func tlsMinVersion() uint16 {
	if strings.EqualFold(os.Getenv("DECREE_TLS_MIN_VERSION"), "TLS12") {
		return tls.VersionTLS12
	}
	return tls.VersionTLS13
}

// tls12CipherSuites is an explicit allowlist used when TLS 1.2 is enabled.
// All suites use ECDHE key exchange and AEAD encryption; RC4, CBC, and
// static RSA key exchange are excluded.
var tls12CipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
}

// certLoader returns a GetCertificate callback that reloads the cert+key pair
// from disk on every TLS handshake, enabling zero-downtime certificate rotation.
func certLoader(certFile, keyFile string) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("reload cert: %w", err)
		}
		return &cert, nil
	}
}

func loadCAPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificates parsed from %s", path)
	}
	return pool, nil
}
