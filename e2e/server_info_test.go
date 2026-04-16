//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerInfo(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	info, err := admin.GetServerInfo(ctx)
	require.NoError(t, err)

	// Version and commit are set at build time.
	assert.NotEmpty(t, info.Version)

	// Docker-compose enables all services and usage tracking, no JWT.
	assert.True(t, info.Features["schema"], "schema should be enabled")
	assert.True(t, info.Features["config"], "config should be enabled")
	assert.True(t, info.Features["audit"], "audit should be enabled")
	assert.True(t, info.Features["usage_tracking"], "usage tracking should be enabled")
	assert.False(t, info.Features["jwt_auth"], "jwt auth should be disabled in e2e")
	assert.True(t, info.Features["http_gateway"], "http gateway should be enabled")
}
