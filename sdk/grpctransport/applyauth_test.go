package grpctransport

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestApplyAuth_BearerToken_SetsAuthorizationHeader(t *testing.T) {
	ctx, err := applyAuth(context.Background(), authConfig{bearerToken: "mytoken"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("no outgoing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) != 1 || vals[0] != "Bearer mytoken" {
		t.Errorf("got authorization %v, want [Bearer mytoken]", vals)
	}
}

func TestApplyAuth_BearerToken_NoMetadataHeaders(t *testing.T) {
	ctx, err := applyAuth(context.Background(), authConfig{
		bearerToken: "tok",
		subject:     "alice",
		role:        "admin",
		tenantID:    "acme",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("no outgoing metadata")
	}
	if len(md.Get("x-subject")) > 0 {
		t.Error("x-subject must not be set when bearerToken is present")
	}
	if len(md.Get("x-role")) > 0 {
		t.Error("x-role must not be set when bearerToken is present")
	}
	if len(md.Get("x-tenant-id")) > 0 {
		t.Error("x-tenant-id must not be set when bearerToken is present")
	}
}

func TestApplyAuth_TokenSource_CallsSourceAndSetsHeader(t *testing.T) {
	called := false
	src := func(context.Context) (string, error) {
		called = true
		return "dynamic-tok", nil
	}
	ctx, err := applyAuth(context.Background(), authConfig{tokenSource: src})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("token source was not called")
	}
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("no outgoing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) != 1 || vals[0] != "Bearer dynamic-tok" {
		t.Errorf("got authorization %v, want [Bearer dynamic-tok]", vals)
	}
}

func TestApplyAuth_TokenSource_ErrorPropagates(t *testing.T) {
	want := errors.New("refresh failed")
	_, err := applyAuth(context.Background(), authConfig{
		tokenSource: func(context.Context) (string, error) { return "", want },
	})
	if !errors.Is(err, want) {
		t.Errorf("got %v, want %v", err, want)
	}
}

func TestApplyAuth_TokenSource_TakesPrecedenceOverBearerToken(t *testing.T) {
	ctx, err := applyAuth(context.Background(), authConfig{
		tokenSource: func(context.Context) (string, error) { return "source-tok", nil },
		bearerToken: "static-tok",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	vals := md.Get("authorization")
	if len(vals) != 1 || vals[0] != "Bearer source-tok" {
		t.Errorf("got authorization %v, want tokenSource to win", vals)
	}
}

func TestApplyAuth_MetadataHeaders_AllFields(t *testing.T) {
	ctx, err := applyAuth(context.Background(), authConfig{
		subject:  "alice",
		role:     "admin",
		tenantID: "acme",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("no outgoing metadata")
	}
	check := func(key, want string) {
		t.Helper()
		vals := md.Get(key)
		if len(vals) != 1 || vals[0] != want {
			t.Errorf("%s: got %v, want [%s]", key, vals, want)
		}
	}
	check("x-subject", "alice")
	check("x-role", "admin")
	check("x-tenant-id", "acme")
}

func TestApplyAuth_MetadataHeaders_OnlyRoleSet(t *testing.T) {
	ctx, err := applyAuth(context.Background(), authConfig{role: "viewer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("no outgoing metadata")
	}
	if vals := md.Get("x-role"); len(vals) != 1 || vals[0] != "viewer" {
		t.Errorf("x-role: got %v, want [viewer]", vals)
	}
	if len(md.Get("x-subject")) > 0 {
		t.Error("x-subject must be absent when not set")
	}
	if len(md.Get("x-tenant-id")) > 0 {
		t.Error("x-tenant-id must be absent when not set")
	}
}

func TestApplyAuth_EmptyConfig_SetsNoHeaders(t *testing.T) {
	ctx, err := applyAuth(context.Background(), authConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return // no metadata at all is fine
	}
	for _, key := range []string{"authorization", "x-subject", "x-role", "x-tenant-id"} {
		if len(md.Get(key)) > 0 {
			t.Errorf("key %s should be absent for empty config", key)
		}
	}
}
