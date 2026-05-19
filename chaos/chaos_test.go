//go:build chaos

package chaos

import (
	"context"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

// --- Service address ---

func serviceAddr() string {
	if v := os.Getenv("SERVICE_ADDR"); v != "" {
		return v
	}
	return "localhost:9090"
}

// --- Container names (configurable via env for non-default compose projects) ---

func postgresContainer() string {
	if v := os.Getenv("POSTGRES_CONTAINER"); v != "" {
		return v
	}
	return "decree-postgres-1"
}

func redisContainer() string {
	if v := os.Getenv("REDIS_CONTAINER"); v != "" {
		return v
	}
	return "decree-redis-1"
}

func serverContainer() string {
	if v := os.Getenv("SERVER_CONTAINER"); v != "" {
		return v
	}
	return "decree-service-1"
}

// --- gRPC client helpers ---

func dial(t testing.TB) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(serviceAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func newAdminClient(conn *grpc.ClientConn) *adminclient.Client {
	c, err := grpctransport.NewAdminClient(conn,
		grpctransport.WithSubject("chaos-test"),
		grpctransport.WithRole("superadmin"))
	if err != nil {
		panic(err)
	}
	return c
}

func newConfigClient(conn *grpc.ClientConn) *configclient.Client {
	c, err := grpctransport.NewConfigClient(conn,
		grpctransport.WithSubject("chaos-test"),
		grpctransport.WithRole("superadmin"))
	if err != nil {
		panic(err)
	}
	return c
}

func newRawConfigClient(conn *grpc.ClientConn) pb.ConfigServiceClient {
	return pb.NewConfigServiceClient(conn)
}

// --- Container control (docker CLI, zero new deps) ---

func containerPause(t testing.TB, name string) {
	t.Helper()
	runDocker(t, "pause", name)
}

func containerUnpause(t testing.TB, name string) {
	t.Helper()
	runDocker(t, "unpause", name)
}

func containerStop(t testing.TB, name string) {
	t.Helper()
	runDocker(t, "stop", "--time", "5", name)
}

func containerStart(t testing.TB, name string) {
	t.Helper()
	runDocker(t, "start", name)
}

func containerSignal(t testing.TB, name, sig string) {
	t.Helper()
	runDocker(t, "kill", "--signal", sig, name)
}

func runDocker(t testing.TB, args ...string) {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	require.NoError(t, err, "docker %v: %s", args, out)
}

// --- Wait helpers ---

// waitReachable polls the service TCP port until it accepts connections.
func waitReachable(t testing.TB, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", serviceAddr(), time.Second)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("service at %s not reachable within %v", serviceAddr(), timeout)
}

// waitContainerHealthy polls docker inspect until health status is "healthy".
func waitContainerHealthy(t testing.TB, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("docker", "inspect",
			"--format={{.State.Health.Status}}", name).Output()
		if err == nil && string(out) == "healthy\n" {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("container %s not healthy within %v", name, timeout)
}

// eventually polls f every 500ms until it returns true or timeout elapses.
func eventually(t testing.TB, timeout time.Duration, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("condition not satisfied within %v", timeout)
}

// --- Fixtures ---

func makeSchema(t testing.TB, admin *adminclient.Client, name string) (string, func()) {
	t.Helper()
	ctx := context.Background()
	s, err := admin.CreateSchema(ctx, name, []adminclient.Field{
		{Path: "chaos.field0", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)
	return s.ID, func() { _ = admin.DeleteSchema(context.Background(), s.ID) }
}

func makeTenant(t testing.TB, admin *adminclient.Client, name, schemaID string) (string, func()) {
	t.Helper()
	tenant, err := admin.CreateTenant(context.Background(), name, schemaID, 1)
	require.NoError(t, err)
	return tenant.ID, func() { _ = admin.DeleteTenant(context.Background(), tenant.ID) }
}
