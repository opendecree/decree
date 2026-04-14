package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeChecksum_Deterministic(t *testing.T) {
	c1 := computeChecksum("hello world")
	c2 := computeChecksum("hello world")
	assert.Equal(t, c1, c2)
}

func TestComputeChecksum_DifferentValues(t *testing.T) {
	c1 := computeChecksum("hello")
	c2 := computeChecksum("world")
	assert.NotEqual(t, c1, c2)
}

func TestPtrString_Empty(t *testing.T) {
	assert.Nil(t, ptrString(""))
}

func TestPtrString_NonEmpty(t *testing.T) {
	s := ptrString("test")
	require.NotNil(t, s)
	assert.Equal(t, "test", *s)
}
