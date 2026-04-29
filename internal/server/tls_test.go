package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// certBundle holds a CA + an end-entity keypair signed by it.
type certBundle struct {
	caCertPEM   []byte
	leafCertPEM []byte
	leafKeyPEM  []byte
}

// writeFiles writes the bundle to dir and returns the file paths.
func (b certBundle) writeFiles(t *testing.T, dir, prefix string) (caFile, certFile, keyFile string) {
	t.Helper()
	caFile = filepath.Join(dir, prefix+"_ca.pem")
	certFile = filepath.Join(dir, prefix+"_cert.pem")
	keyFile = filepath.Join(dir, prefix+"_key.pem")
	require.NoError(t, os.WriteFile(caFile, b.caCertPEM, 0o600))
	require.NoError(t, os.WriteFile(certFile, b.leafCertPEM, 0o600))
	require.NoError(t, os.WriteFile(keyFile, b.leafKeyPEM, 0o600))
	return
}

// genCertBundle generates a self-signed CA and an end-entity cert/key signed
// by it. The leaf cert lists 127.0.0.1 + ::1 + "localhost" as SANs.
func genCertBundle(t *testing.T, commonName string, isClient bool) certBundle {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA " + commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	if isClient {
		leafTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		leafTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		leafTmpl.DNSNames = []string{"localhost"}
		leafTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caTmpl, &leafKey.PublicKey, caKey)
	require.NoError(t, err)

	leafKeyDER, err := x509.MarshalECPrivateKey(leafKey)
	require.NoError(t, err)

	return certBundle{
		caCertPEM:   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		leafCertPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		leafKeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER}),
	}
}

// startTLSServer starts a gRPC server with the given TLS config and returns
// its listen address + cleanup func.
func startTLSServer(t *testing.T, tlsCfg *TLSConfig) (string, func()) {
	t.Helper()
	srv, err := New("0", &noopInterceptor{},
		WithLogger(slog.Default()),
		WithTLS(tlsCfg),
	)
	require.NoError(t, err)
	addr := srv.listener.Addr().String()
	go func() { _ = srv.Serve(context.Background()) }()
	return addr, func() { srv.GracefulStop(context.Background()) }
}

func dialHealth(ctx context.Context, addr string, opt grpc.DialOption) error {
	conn, err := grpc.NewClient(addr, opt)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	return err
}

func TestTLSConfig_RequiresCertAndKey(t *testing.T) {
	_, err := TLSConfig{}.ServerCredentials()
	require.Error(t, err)
}

func TestTLS_RejectsPlaintextDial(t *testing.T) {
	dir := t.TempDir()
	server := genCertBundle(t, "localhost", false)
	_, srvCert, srvKey := server.writeFiles(t, dir, "srv")

	addr, stop := startTLSServer(t, &TLSConfig{CertFile: srvCert, KeyFile: srvKey})
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := dialHealth(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.Error(t, err, "plaintext dial against TLS server must fail")
}

func TestTLS_AcceptsTLSDial(t *testing.T) {
	dir := t.TempDir()
	server := genCertBundle(t, "localhost", false)
	caFile, srvCert, srvKey := server.writeFiles(t, dir, "srv")

	addr, stop := startTLSServer(t, &TLSConfig{CertFile: srvCert, KeyFile: srvKey})
	defer stop()

	creds, err := GatewayTLSConfig{CAFile: caFile, ServerName: "localhost"}.ClientCredentials()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = dialHealth(ctx, addr, grpc.WithTransportCredentials(creds))
	require.NoError(t, err)
}

func TestMTLS_AcceptsSignedClient(t *testing.T) {
	dir := t.TempDir()
	server := genCertBundle(t, "localhost", false)
	srvCAFile, srvCert, srvKey := server.writeFiles(t, dir, "srv")

	client := genCertBundle(t, "client", true)
	clientCAFile, clientCert, clientKey := client.writeFiles(t, dir, "cli")

	addr, stop := startTLSServer(t, &TLSConfig{
		CertFile:     srvCert,
		KeyFile:      srvKey,
		ClientCAFile: clientCAFile,
	})
	defer stop()

	creds, err := GatewayTLSConfig{
		CAFile:         srvCAFile,
		ServerName:     "localhost",
		ClientCertFile: clientCert,
		ClientKeyFile:  clientKey,
	}.ClientCredentials()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = dialHealth(ctx, addr, grpc.WithTransportCredentials(creds))
	require.NoError(t, err, "mTLS dial with signed client cert must succeed")
}

func TestMTLS_RejectsUnsignedClient(t *testing.T) {
	dir := t.TempDir()
	server := genCertBundle(t, "localhost", false)
	srvCAFile, srvCert, srvKey := server.writeFiles(t, dir, "srv")

	clientCA := genCertBundle(t, "trusted", true)
	clientCAFile := filepath.Join(dir, "trusted_ca.pem")
	require.NoError(t, os.WriteFile(clientCAFile, clientCA.caCertPEM, 0o600))

	// Foreign client cert — signed by an untrusted CA.
	foreign := genCertBundle(t, "foreign", true)
	_, foreignCert, foreignKey := foreign.writeFiles(t, dir, "foreign")

	addr, stop := startTLSServer(t, &TLSConfig{
		CertFile:     srvCert,
		KeyFile:      srvKey,
		ClientCAFile: clientCAFile,
	})
	defer stop()

	creds, err := GatewayTLSConfig{
		CAFile:         srvCAFile,
		ServerName:     "localhost",
		ClientCertFile: foreignCert,
		ClientKeyFile:  foreignKey,
	}.ClientCredentials()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = dialHealth(ctx, addr, grpc.WithTransportCredentials(creds))
	require.Error(t, err, "mTLS dial with foreign client cert must fail")
}

func TestGatewayTLSConfig_RequiresCertKeyPair(t *testing.T) {
	_, err := GatewayTLSConfig{ClientCertFile: "x"}.ClientCredentials()
	require.Error(t, err)
	_, err = GatewayTLSConfig{ClientKeyFile: "y"}.ClientCredentials()
	require.Error(t, err)
}
