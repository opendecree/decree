//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

func serviceAddr() string {
	if addr := os.Getenv("SERVICE_ADDR"); addr != "" {
		return addr
	}
	return "localhost:9090"
}

func dial(t *testing.T) *grpc.ClientConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, serviceAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err, "failed to connect to service at %s", serviceAddr())
	t.Cleanup(func() { conn.Close() })
	return conn
}

func newAdminClient(conn *grpc.ClientConn) *adminclient.Client {
	return grpctransport.NewAdminClient(conn, grpctransport.WithSubject("e2e-test"))
}

func newConfigClient(conn *grpc.ClientConn) *configclient.Client {
	return grpctransport.NewConfigClient(conn, grpctransport.WithSubject("e2e-test"))
}

func ptr[T any](v T) *T { return &v }
