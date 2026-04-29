package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

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
func (c TLSConfig) ServerCredentials() (credentials.TransportCredentials, error) {
	if c.CertFile == "" || c.KeyFile == "" {
		return nil, errors.New("TLS cert and key files are required")
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server keypair: %w", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
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
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
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
