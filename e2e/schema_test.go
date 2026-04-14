//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeevdr/central-config-service/sdk/adminclient"
)

// --- Schema Lifecycle (adminclient) ---

func TestSchemaLifecycle(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	// Create schema with fields.
	s, err := admin.CreateSchema(ctx, "payments-e2e", []adminclient.Field{
		{Path: "payments.fee", Type: "FIELD_TYPE_STRING", Description: "Transaction fee percentage"},
		{Path: "payments.currency", Type: "FIELD_TYPE_STRING", Description: "Default currency"},
		{Path: "payments.timeout", Type: "FIELD_TYPE_DURATION", Description: "Payment timeout"},
	}, "E2E test schema")
	require.NoError(t, err)
	assert.NotEmpty(t, s.ID)
	assert.Equal(t, "payments-e2e", s.Name)
	assert.Equal(t, int32(1), s.Version)
	assert.False(t, s.Published)
	assert.Len(t, s.Fields, 3)

	// Get schema (latest).
	got, err := admin.GetSchema(ctx, s.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), got.Version)

	// List schemas.
	schemas, err := admin.ListSchemas(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(schemas), 1)

	// Update schema — add a field, creates v2.
	updated, err := admin.UpdateSchema(ctx, s.ID, []adminclient.Field{
		{Path: "payments.max_retries", Type: "FIELD_TYPE_INT", Description: "Max retry count"},
	}, nil, "Add max_retries field")
	require.NoError(t, err)
	assert.Equal(t, int32(2), updated.Version)
	assert.Len(t, updated.Fields, 4)

	// Get specific version.
	v1, err := admin.GetSchemaVersion(ctx, s.ID, 1)
	require.NoError(t, err)
	assert.Len(t, v1.Fields, 3)

	// Publish v1.
	published, err := admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)
	assert.True(t, published.Published)

	// Publish already-published → should error.
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	if err != nil {
		assert.ErrorIs(t, err, adminclient.ErrFailedPrecondition)
	}

	// Cleanup.
	require.NoError(t, admin.DeleteSchema(ctx, s.ID))

	_, err = admin.GetSchema(ctx, s.ID)
	assert.ErrorIs(t, err, adminclient.ErrNotFound)
}

// --- Schema Export/Import (adminclient) ---

func TestSchemaExportImport(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	// 1. Create a schema with fields.
	s, err := admin.CreateSchema(ctx, "export-e2e", []adminclient.Field{
		{Path: "trade.fee", Type: "FIELD_TYPE_STRING", Description: "Fee percentage"},
		{Path: "trade.currency", Type: "FIELD_TYPE_STRING"},
		{Path: "trade.timeout", Type: "FIELD_TYPE_DURATION"},
	}, "Schema for export testing")
	require.NoError(t, err)

	// 2. Export.
	yamlContent, err := admin.ExportSchema(ctx, s.ID, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, yamlContent)

	yamlStr := string(yamlContent)
	t.Logf("Exported YAML:\n%s", yamlStr)
	assert.Contains(t, yamlStr, "syntax:")
	assert.Contains(t, yamlStr, "name: export-e2e")
	assert.Contains(t, yamlStr, "trade.fee")

	// 3. Import identical YAML → should get AlreadyExists.
	_, err = admin.ImportSchema(ctx, yamlContent)
	assert.ErrorIs(t, err, adminclient.ErrAlreadyExists)

	// 4. Modify YAML — add a field, re-import.
	modified := bytes.Replace(yamlContent,
		[]byte("    trade.timeout:"),
		[]byte("    trade.max_retries:\n        type: integer\n    trade.timeout:"),
		1,
	)
	imported, err := admin.ImportSchema(ctx, modified)
	require.NoError(t, err)
	assert.Equal(t, int32(2), imported.Version)
	assert.Len(t, imported.Fields, 4)
	assert.False(t, imported.Published)

	// 5. Verify via GetSchema.
	got, err := admin.GetSchema(ctx, s.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(2), got.Version)

	// 6. Import new schema by name.
	newYAML := []byte(`syntax: "v1"
name: brand-new-e2e
description: Created via import
fields:
  config.enabled:
    type: string
    default: "true"
`)
	newSchema, err := admin.ImportSchema(ctx, newYAML)
	require.NoError(t, err)
	assert.Equal(t, "brand-new-e2e", newSchema.Name)
	assert.Equal(t, int32(1), newSchema.Version)

	// Cleanup.
	_ = admin.DeleteSchema(ctx, s.ID)
	_ = admin.DeleteSchema(ctx, newSchema.ID)
}
