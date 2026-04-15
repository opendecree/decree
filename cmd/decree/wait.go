package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// waitForServer polls the gRPC health check endpoint until the server is
// ready or the timeout expires. Uses exponential backoff starting at 500ms.
func waitForServer(conn *grpc.ClientConn, timeout time.Duration) error {
	client := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	backoff := 500 * time.Millisecond
	maxBackoff := 5 * time.Second

	for {
		resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
		if err == nil && resp.Status == healthpb.HealthCheckResponse_SERVING {
			return nil
		}

		select {
		case <-ctx.Done():
			if err != nil {
				return fmt.Errorf("server not ready after %s: %w", timeout, err)
			}
			return fmt.Errorf("server not ready after %s: status %s", timeout, resp.Status)
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, maxBackoff)
	}
}
