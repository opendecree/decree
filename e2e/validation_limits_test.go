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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
)

// docker-compose pins the server to:
//
//	SCHEMA_MAX_FIELDS=100
//	SCHEMA_MAX_DOC_BYTES=4096
//	SCHEMA_MAX_REMOVE_FIELDS=5
//	CONFIG_MAX_LIST_LEN=5
//
// so these tests can trip each limit with small payloads.
const (
	maxFields        = 100
	maxDocBytes      = 4096
	maxRemoveFields  = 5
	maxConfigListLen = 5
)

// superadminMD returns outgoing metadata that satisfies the server's auth
// interceptor for e2e calls made directly via the proto client.
func superadminMD(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-subject", "e2e-limits", "x-role", "superadmin")
}

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
			Type: adminclient.FieldTypeString,
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
			Type: adminclient.FieldTypeString,
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

// --- Config list-length limits (CONFIG_MAX_LIST_LEN=5 in docker-compose) ---

// TestValidationLimits_GetFields_ExceedsListLen verifies that GetFields
// rejects a request with more than CONFIG_MAX_LIST_LEN field_paths.
func TestValidationLimits_GetFields_ExceedsListLen(t *testing.T) {
	conn := dial(t)
	cfgSvc := pb.NewConfigServiceClient(conn)
	ctx := superadminMD(context.Background())

	paths := make([]string, maxConfigListLen+1)
	for i := range paths {
		paths[i] = fmt.Sprintf("f.field_%d", i)
	}

	_, err := cfgSvc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   "00000000-0000-0000-0000-000000000099",
		FieldPaths: paths,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), fmt.Sprintf("exceeds limit of %d", maxConfigListLen))
}

// TestValidationLimits_GetFields_AtLimitAccepted verifies that GetFields
// accepts a request with exactly CONFIG_MAX_LIST_LEN field_paths.
func TestValidationLimits_GetFields_AtLimitAccepted(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfgSvc := pb.NewConfigServiceClient(conn)
	ctx := context.Background()
	authCtx := superadminMD(ctx)

	fields := make([]adminclient.Field, maxConfigListLen)
	for i := range fields {
		fields[i] = adminclient.Field{Path: fmt.Sprintf("f.field_%d", i), Type: adminclient.FieldTypeString}
	}
	s, err := admin.CreateSchema(ctx, fmt.Sprintf("limits-get-%s", randSuffix()), fields, "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteSchema(ctx, s.ID) })
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, fmt.Sprintf("limits-get-t-%s", randSuffix()), s.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteTenant(ctx, tenant.ID) })

	paths := make([]string, maxConfigListLen)
	for i := range paths {
		paths[i] = fmt.Sprintf("f.field_%d", i)
	}
	_, err = cfgSvc.GetFields(authCtx, &pb.GetFieldsRequest{TenantId: tenant.ID, FieldPaths: paths})
	// Any error here is fine — but must NOT be InvalidArgument (limit not tripped).
	if err != nil {
		assert.NotEqual(t, codes.InvalidArgument, status.Code(err),
			"limit guard must not fire at exactly %d paths", maxConfigListLen)
	}
}

// TestValidationLimits_SetFields_ExceedsListLen verifies that SetFields
// rejects a request with more than CONFIG_MAX_LIST_LEN updates.
func TestValidationLimits_SetFields_ExceedsListLen(t *testing.T) {
	conn := dial(t)
	cfgSvc := pb.NewConfigServiceClient(conn)
	ctx := superadminMD(context.Background())

	updates := make([]*pb.FieldUpdate, maxConfigListLen+1)
	for i := range updates {
		updates[i] = &pb.FieldUpdate{FieldPath: fmt.Sprintf("f.field_%d", i)}
	}

	_, err := cfgSvc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: "00000000-0000-0000-0000-000000000099",
		Updates:  updates,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), fmt.Sprintf("exceeds limit of %d", maxConfigListLen))
}

// TestValidationLimits_Subscribe_ExceedsListLen verifies that Subscribe
// rejects a request with more than CONFIG_MAX_LIST_LEN field_paths.
func TestValidationLimits_Subscribe_ExceedsListLen(t *testing.T) {
	conn := dial(t)
	cfgSvc := pb.NewConfigServiceClient(conn)
	ctx := superadminMD(context.Background())

	paths := make([]string, maxConfigListLen+1)
	for i := range paths {
		paths[i] = fmt.Sprintf("f.field_%d", i)
	}

	stream, err := cfgSvc.Subscribe(ctx, &pb.SubscribeRequest{
		TenantId:   "00000000-0000-0000-0000-000000000099",
		FieldPaths: paths,
	})
	require.NoError(t, err, "Subscribe RPC setup must not fail before the first Recv")

	_, err = stream.Recv()
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), fmt.Sprintf("exceeds limit of %d", maxConfigListLen))
}

// --- Schema remove_fields limit (SCHEMA_MAX_REMOVE_FIELDS=5 in docker-compose) ---

// TestValidationLimits_UpdateSchema_RemoveFields_ExceedsListLen verifies
// that UpdateSchema rejects a remove_fields list longer than
// SCHEMA_MAX_REMOVE_FIELDS.
func TestValidationLimits_UpdateSchema_RemoveFields_ExceedsListLen(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	schemaSvc := pb.NewSchemaServiceClient(conn)
	ctx := context.Background()
	authCtx := superadminMD(ctx)

	fields := make([]adminclient.Field, maxRemoveFields+1)
	for i := range fields {
		fields[i] = adminclient.Field{Path: fmt.Sprintf("f.field_%d", i), Type: adminclient.FieldTypeString}
	}
	s, err := admin.CreateSchema(ctx, fmt.Sprintf("limits-rm-%s", randSuffix()), fields, "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.DeleteSchema(ctx, s.ID) })

	removePaths := make([]string, maxRemoveFields+1)
	for i := range removePaths {
		removePaths[i] = fmt.Sprintf("f.field_%d", i)
	}

	_, err = schemaSvc.UpdateSchema(authCtx, &pb.UpdateSchemaRequest{
		Id:           s.ID,
		RemoveFields: removePaths,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), fmt.Sprintf("exceeds limit of %d", maxRemoveFields))
}
