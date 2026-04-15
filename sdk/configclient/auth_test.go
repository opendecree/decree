package configclient

import (
	"context"
	"reflect"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestWithSubject(t *testing.T) {
	c := New(nil, WithSubject("alice"))
	if got := c.opts.subject; got != "alice" {
		t.Errorf("got %v, want %v", got, "alice")
	}
}

func TestWithRole(t *testing.T) {
	c := New(nil, WithRole("admin"))
	if got := c.opts.role; got != "admin" {
		t.Errorf("got %v, want %v", got, "admin")
	}
}

func TestWithRole_Default(t *testing.T) {
	c := New(nil)
	if got := c.opts.role; got != "superadmin" {
		t.Errorf("got %v, want %v", got, "superadmin")
	}
}

func TestWithTenantID(t *testing.T) {
	c := New(nil, WithTenantID("t1"))
	if got := c.opts.tenantID; got != "t1" {
		t.Errorf("got %v, want %v", got, "t1")
	}
}

func TestWithBearerToken(t *testing.T) {
	c := New(nil, WithBearerToken("jwt-token"))
	if got := c.opts.bearerToken; got != "jwt-token" {
		t.Errorf("got %v, want %v", got, "jwt-token")
	}
}

func TestWithAuth_MetadataHeaders(t *testing.T) {
	c := New(nil, WithSubject("alice"), WithRole("admin"), WithTenantID("t1"))
	ctx := c.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata to be present")
	}
	if got, want := md.Get("x-subject"), []string{"alice"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := md.Get("x-role"), []string{"admin"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := md.Get("x-tenant-id"), []string{"t1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWithAuth_BearerTokenOverridesMetadata(t *testing.T) {
	c := New(nil, WithSubject("alice"), WithBearerToken("jwt-token"))
	ctx := c.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata to be present")
	}
	if got, want := md.Get("authorization"), []string{"Bearer jwt-token"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := md.Get("x-subject"); len(got) != 0 {
		t.Errorf("expected empty x-subject when bearer token is used, got %v", got)
	}
}

func TestWithAuth_NoOptions(t *testing.T) {
	c := New(nil, WithRole("")) // clear default role
	c.opts.subject = ""
	c.opts.tenantID = ""
	ctx := c.withAuth(context.Background())

	_, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		t.Error("expected no metadata when all options are empty")
	}
}
