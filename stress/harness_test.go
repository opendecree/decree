//go:build stress

package stress

import (
	"context"
	"fmt"
	"os"
	"testing"

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

func dial(tb testing.TB) *grpc.ClientConn {
	tb.Helper()
	conn, err := grpc.NewClient(serviceAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(tb, err)
	tb.Cleanup(func() { conn.Close() })
	return conn
}

func newAdmin(conn *grpc.ClientConn) *adminclient.Client {
	return grpctransport.NewAdminClient(conn, grpctransport.WithSubject("stress-test"))
}

func newConfig(conn *grpc.ClientConn) *configclient.Client {
	return grpctransport.NewConfigClient(conn, grpctransport.WithSubject("stress-test"))
}

// makeSchema creates and publishes a schema with fieldCount string fields.
// Returns the schema ID and a cleanup func.
func makeSchema(tb testing.TB, admin *adminclient.Client, name string, fieldCount int) (string, func()) {
	tb.Helper()
	ctx := context.Background()
	fields := make([]adminclient.Field, fieldCount)
	for i := range fields {
		fields[i] = adminclient.Field{
			Path: fmt.Sprintf("f.field_%d", i),
			Type: "FIELD_TYPE_STRING",
		}
	}
	s, err := admin.CreateSchema(ctx, name, fields, "")
	require.NoError(tb, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(tb, err)
	return s.ID, func() { _ = admin.DeleteSchema(ctx, s.ID) }
}

// makeTenant creates a tenant under the given schema and returns its ID + cleanup.
func makeTenant(tb testing.TB, admin *adminclient.Client, name, schemaID string) (string, func()) {
	tb.Helper()
	ctx := context.Background()
	tenant, err := admin.CreateTenant(ctx, name, schemaID, 1)
	require.NoError(tb, err)
	return tenant.ID, func() { _ = admin.DeleteTenant(ctx, tenant.ID) }
}
