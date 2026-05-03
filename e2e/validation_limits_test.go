//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/sdk/adminclient"
)

// docker-compose pins the server to:
//
//	SCHEMA_MAX_FIELDS=100
//	SCHEMA_MAX_DOC_BYTES=4096
//
// so these tests can trip each limit with small payloads.
const (
	maxFields   = 100
	maxDocBytes = 4096
)

// TestValidationLimits_MaxFieldsRejected: CreateSchema with more fields
// than the configured cap is rejected with InvalidArgument and a message
// citing the limit.
func TestValidationLimits_MaxFieldsRejected(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	fields := make([]adminclient.Field, maxFields+1)
	for i := range fields {
		fields[i] = adminclient.Field{
			Path: fmt.Sprintf("f.field_%d", i),
			Type: "FIELD_TYPE_STRING",
		}
	}

	_, err := admin.CreateSchema(ctx, fmt.Sprintf("limits-fields-%s", randSuffix()), fields, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, adminclient.ErrInvalidArgument))
	assert.Contains(t, err.Error(), fmt.Sprintf("exceeds limit of %d", maxFields))
}

// TestValidationLimits_MaxFieldsAtLimitAccepted: CreateSchema with
// exactly maxFields entries succeeds — the cap is inclusive at the limit
// and exclusive only above it.
func TestValidationLimits_MaxFieldsAtLimitAccepted(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	fields := make([]adminclient.Field, maxFields)
	for i := range fields {
		fields[i] = adminclient.Field{
			Path: fmt.Sprintf("f.field_%d", i),
			Type: "FIELD_TYPE_STRING",
		}
	}

	schemaName := fmt.Sprintf("limits-fields-ok-%s", randSuffix())
	s, err := admin.CreateSchema(ctx, schemaName, fields, "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteSchema(ctx, s.ID) })
}

// TestValidationLimits_MaxDocBytesRejected: ImportSchema with a YAML body
// larger than SCHEMA_MAX_DOC_BYTES is rejected with InvalidArgument and a
// message citing the limit.
func TestValidationLimits_MaxDocBytesRejected(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	// Build a syntactically valid YAML, then pad with a large comment so
	// the byte count exceeds the cap. The pre-parse byte check fires
	// before the YAML parser cares about comment size.
	header := fmt.Sprintf(`spec_version: "v1"
name: limits-bytes-%s
fields:
  app.name:
    type: string
`, randSuffix())
	padding := "\n# " + strings.Repeat("x", maxDocBytes)
	yaml := []byte(header + padding)
	require.Greater(t, len(yaml), maxDocBytes, "test fixture must exceed the cap")

	_, err := admin.ImportSchema(ctx, yaml)
	require.Error(t, err)
	assert.True(t, errors.Is(err, adminclient.ErrInvalidArgument))
	assert.Contains(t, err.Error(), fmt.Sprintf("exceeds limit of %d", maxDocBytes))
}
