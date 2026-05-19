package grpctransport_test

import (
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/grpctransport"
)

func conn() grpc.ClientConnInterface {
	c, _ := grpc.NewClient("passthrough:///localhost:9999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	return c
}

func TestBuildConfig_NoRole_Errors(t *testing.T) {
	_, err := grpctransport.NewConfigTransport(conn())
	if err == nil {
		t.Fatal("expected error when no role or bearer token provided")
	}
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Fatalf("expected ErrRoleRequired, got %v", err)
	}
}

func TestBuildConfig_WithRole_OK(t *testing.T) {
	_, err := grpctransport.NewConfigTransport(conn(),
		grpctransport.WithRole("user"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildConfig_WithBearerToken_NoRoleRequired(t *testing.T) {
	_, err := grpctransport.NewConfigTransport(conn(),
		grpctransport.WithBearerToken("tok"),
	)
	if err != nil {
		t.Fatalf("unexpected error with bearer token and no role: %v", err)
	}
}

func TestNewAdminClient_NoRole_Errors(t *testing.T) {
	_, err := grpctransport.NewAdminClient(conn())
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Fatalf("expected ErrRoleRequired, got %v", err)
	}
}

func TestNewWatcher_NoRole_Errors(t *testing.T) {
	_, err := grpctransport.NewWatcher(conn(), "tenant-id")
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Fatalf("expected ErrRoleRequired, got %v", err)
	}
}
