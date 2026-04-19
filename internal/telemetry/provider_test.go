package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_Disabled_ReturnsNoopShutdown(t *testing.T) {
	shutdown, err := Init(context.Background(), Config{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// No-op shutdown should succeed with a nil context and not panic.
	assert.NoError(t, shutdown(context.Background()))
}
