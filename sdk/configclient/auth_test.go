package configclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestWithSubject(t *testing.T) {
	c := New(nil, WithSubject("alice"))
	assert.Equal(t, "alice", c.opts.subject)
}

func TestWithRole(t *testing.T) {
	c := New(nil, WithRole("admin"))
	assert.Equal(t, "admin", c.opts.role)
}

func TestWithRole_Default(t *testing.T) {
	c := New(nil)
	assert.Equal(t, "superadmin", c.opts.role)
}

func TestWithTenantID(t *testing.T) {
	c := New(nil, WithTenantID("t1"))
	assert.Equal(t, "t1", c.opts.tenantID)
}

func TestWithBearerToken(t *testing.T) {
	c := New(nil, WithBearerToken("jwt-token"))
	assert.Equal(t, "jwt-token", c.opts.bearerToken)
}

func TestWithAuth_MetadataHeaders(t *testing.T) {
	c := New(nil, WithSubject("alice"), WithRole("admin"), WithTenantID("t1"))
	ctx := c.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"alice"}, md.Get("x-subject"))
	assert.Equal(t, []string{"admin"}, md.Get("x-role"))
	assert.Equal(t, []string{"t1"}, md.Get("x-tenant-id"))
}

func TestWithAuth_BearerTokenOverridesMetadata(t *testing.T) {
	c := New(nil, WithSubject("alice"), WithBearerToken("jwt-token"))
	ctx := c.withAuth(context.Background())

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"Bearer jwt-token"}, md.Get("authorization"))
	assert.Empty(t, md.Get("x-subject"), "metadata headers should not be set when bearer token is used")
}

func TestWithAuth_NoOptions(t *testing.T) {
	c := New(nil, WithRole("")) // clear default role
	c.opts.subject = ""
	c.opts.tenantID = ""
	ctx := c.withAuth(context.Background())

	_, ok := metadata.FromOutgoingContext(ctx)
	assert.False(t, ok, "no metadata should be set when all options are empty")
}
