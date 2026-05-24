package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage"
)

func TestNewDB_InvalidDSN(t *testing.T) {
	_, err := storage.NewDB(context.Background(), "not-a-dsn", "")
	require.Error(t, err)
}
