package main

import (
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestWaitForServer_Healthy(t *testing.T) {
	// Start a gRPC server with health check.
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	go srv.Serve(lis)
	defer srv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := waitForServer(conn, 5*time.Second); err != nil {
		t.Fatalf("expected healthy server, got: %v", err)
	}
}

func TestWaitForServer_Timeout(t *testing.T) {
	// Connect to a port that's not listening.
	conn, err := grpc.NewClient("localhost:19999",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	err = waitForServer(conn, 2*time.Second)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
