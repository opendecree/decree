//go:build e2e

package e2e

// Security regression suite for decree#224. Covers the 9 acceptance criteria
// from the security review (decree#26). Each test references the fix issue it
// guards.
//
// docker-compose env vars relied on by this file:
//
//	GRPC_MAX_RECV_MSG_BYTES=65536  (TestSecurity_OversizeRequestRejected)
//	SCHEMA_MAX_FIELDS=100           (TestSecurity_SchemaLimitsEnforced)
//	SCHEMA_MAX_DOC_BYTES=4096       (TestSecurity_SchemaLimitsEnforced)
//	ENABLE_REFLECTION=1             (TestSecurity_ReflectionEnabled)
//	INSECURE_LISTEN=1               (TLS regression at unit level only)

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
)

// grpcMaxRecvBytes must match GRPC_MAX_RECV_MSG_BYTES in docker-compose.yml.
const grpcMaxRecvBytes = 65536

// TestSecurity_OversizeRequestRejected: a gRPC message larger than
// GRPC_MAX_RECV_MSG_BYTES returns codes.ResourceExhausted before any handler
// logic runs. Regression for decree#212.
func TestSecurity_OversizeRequestRejected(t *testing.T) {
	conn := dial(t)
	cfgSvc := pb.NewConfigServiceClient(conn)

	// No auth metadata needed — the transport-level size check fires before
	// any interceptor processes headers.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send a SetFieldRequest with a value string that pushes the encoded
	// proto message well past the 64 KiB server recv cap.
	_, err := cfgSvc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  "00000000-0000-0000-0000-000000000001",
		FieldPath: "app.value",
		Value: &pb.TypedValue{
			Kind: &pb.TypedValue_StringValue{
				StringValue: strings.Repeat("x", grpcMaxRecvBytes+5000),
			},
		},
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err),
		"oversize message must be rejected at transport layer with ResourceExhausted")
}

// TestSecurity_TLSRequired: skipped in the standard e2e suite because
// docker-compose runs the server with INSECURE_LISTEN=1. The equivalent
// unit-level coverage lives in internal/server/server_test.go:
//   - TestNew_RequiresTLSOrInsecure (server refuses to start without TLS or explicit insecure flag)
//   - TestServer_TLS_ClientConnects (TLS handshake succeeds with valid cert)
//
// Regression for decree#213.
func TestSecurity_TLSRequired(t *testing.T) {
	t.Skip("TLS regression covered by internal/server/server_test.go; e2e server runs with INSECURE_LISTEN=1")
}

// TestSecurity_PanicRecovery: skipped in the standard e2e suite because there
// is no public API surface that deliberately panics a handler. The recovery
// interceptor is exercised at unit level in internal/server/interceptors_test.go.
// Regression for decree#214.
func TestSecurity_PanicRecovery(t *testing.T) {
	t.Skip("panic recovery covered by internal/server interceptors unit tests; no public panic-injection surface in e2e")
}

// TestSecurity_SensitiveFieldRedacted: SetField on a sensitive field must
// produce [REDACTED] in the Subscribe event, ExportConfig YAML, and
// QueryWriteLog entry — the plaintext must never appear on any read path.
// Regression for decree#215.
func TestSecurity_SensitiveFieldRedacted(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	cfgSvc := pb.NewConfigServiceClient(conn)
	ctx := context.Background()

	const secretValue = "s3cr3t-t0k3n-abc123"

	// Create schema with one sensitive field.
	s, err := admin.CreateSchema(ctx, "sec-sensitive-"+randSuffix(), []adminclient.Field{
		{Path: "auth.token", Type: "FIELD_TYPE_STRING", Sensitive: true},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)
	tenant, err := admin.CreateTenant(ctx, "sec-sensitive-tenant-"+randSuffix(), s.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = admin.DeleteTenant(context.Background(), tenant.ID)
		_ = admin.DeleteSchema(context.Background(), s.ID)
	})

	// Subscribe before the write so the event is captured.
	subCtx, subCancel := context.WithTimeout(ctx, 10*time.Second)
	defer subCancel()
	subCtx = metadata.AppendToOutgoingContext(subCtx, "x-subject", "e2e-security-sensitive", "x-role", "superadmin")
	stream, err := cfgSvc.Subscribe(subCtx, &pb.SubscribeRequest{
		TenantId:   tenant.ID,
		FieldPaths: []string{"auth.token"},
	})
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Write the secret.
	require.NoError(t, cfg.Set(ctx, tenant.ID, "auth.token", secretValue))

	// --- Subscribe event must not carry plaintext ---
	event, err := stream.Recv()
	require.NoError(t, err)
	newVal := event.GetChange().GetNewValue().GetStringValue()
	assert.Equal(t, "[REDACTED]", newVal, "subscribe event must not carry plaintext sensitive value")
	assert.NotEqual(t, secretValue, newVal)

	// --- ExportConfig must not carry plaintext ---
	exportYAML, err := admin.ExportConfig(ctx, tenant.ID, nil)
	require.NoError(t, err)
	assert.NotContains(t, string(exportYAML), secretValue,
		"ExportConfig must redact sensitive field value")

	// --- QueryWriteLog must not carry plaintext ---
	entries, err := admin.QueryWriteLog(ctx, adminclient.WithAuditTenant(tenant.ID))
	require.NoError(t, err)
	require.NotEmpty(t, entries, "audit log must have at least one entry after Set")
	for _, e := range entries {
		assert.NotEqual(t, secretValue, e.NewValue,
			"audit log new_value must not carry plaintext sensitive value")
		assert.NotEqual(t, secretValue, e.OldValue,
			"audit log old_value must not carry plaintext sensitive value")
	}
}

// TestSecurity_RateLimit: the (burst+1)th rapid request on one tenant returns
// codes.ResourceExhausted; a second tenant's bucket is unaffected.
// Regression for decree#216.
//
// Full per-method and burst-threshold coverage lives in ratelimit_test.go.
func TestSecurity_RateLimit(t *testing.T) {
	a := bootstrapMatrixFixture(t, "sec-rl-a")
	b := bootstrapMatrixFixture(t, "sec-rl-b")
	conn := dial(t)
	cA := scopedClients(t, conn, roleAdmin, a.tenantID)
	cB := scopedClients(t, conn, roleAdmin, b.tenantID)
	ctx := context.Background()

	// Exhaust tenant A's bucket.
	successes, rlErr := burstUntilExhausted(t, func() error {
		_, e := cA.admin.GetSchema(ctx, a.schemaID)
		return e
	})
	assert.Equal(t, codes.ResourceExhausted, status.Code(rlErr),
		"rate limiter must trip with ResourceExhausted")
	assert.GreaterOrEqual(t, successes, ratelimitBurst-1,
		"at least burst-1 requests must succeed before the limiter trips")

	// Tenant B must be unaffected — its bucket is independent.
	_, err := cB.admin.GetSchema(ctx, b.schemaID)
	assert.NoError(t, err, "tenant B must not be rate-limited when tenant A's bucket is exhausted")
}

// TestSecurity_SchemaLimitsEnforced: the three schema-level limits from
// decree#217 are enforced at ingest time.
func TestSecurity_SchemaLimitsEnforced(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	t.Run("too_many_fields", func(t *testing.T) {
		// maxFields+1 fields must be rejected (maxFields matches SCHEMA_MAX_FIELDS).
		fields := make([]adminclient.Field, maxFields+1)
		for i := range fields {
			fields[i] = adminclient.Field{
				Path: fmt.Sprintf("f.field_%d", i),
				Type: "FIELD_TYPE_STRING",
			}
		}
		_, err := admin.CreateSchema(ctx, "sec-limits-fields-"+randSuffix(), fields, "")
		require.Error(t, err)
		assert.ErrorIs(t, err, adminclient.ErrInvalidArgument,
			"schema exceeding max field count must be rejected with InvalidArgument")
	})

	t.Run("oversized_doc", func(t *testing.T) {
		// maxDocBytes+1 YAML must be rejected (maxDocBytes matches SCHEMA_MAX_DOC_BYTES).
		header := fmt.Sprintf("spec_version: \"v1\"\nname: sec-doc-%s\nfields:\n  x:\n    type: string\n", randSuffix())
		yaml := []byte(header + "# " + strings.Repeat("x", maxDocBytes))
		require.Greater(t, len(yaml), maxDocBytes)
		_, err := admin.ImportSchema(ctx, yaml)
		require.Error(t, err)
		assert.ErrorIs(t, err, adminclient.ErrInvalidArgument,
			"schema YAML exceeding max doc size must be rejected with InvalidArgument")
	})

	t.Run("pathological_json_schema", func(t *testing.T) {
		// A JSON Schema with nesting depth > 64 (DefaultLimits.MaxDepth) must
		// not hang the server. The depth pre-scan short-circuits compilation;
		// the server must respond within the test timeout.
		//
		// The constraint is stored at ImportSchema time but the depth check
		// fires lazily in the validator factory. Setting a value must complete
		// quickly (depth check rejects compile; server does not hang).
		deepSchema := buildDeepJSONSchema(66)
		s, err := admin.CreateSchema(ctx, "sec-json-depth-"+randSuffix(), []adminclient.Field{
			{
				Path: "data.payload",
				Type: "FIELD_TYPE_JSON",
				Constraints: &adminclient.FieldConstraints{
					JSONSchema: deepSchema,
				},
			},
		}, "")
		require.NoError(t, err, "CreateSchema with deep json_schema must not error (depth check is lazy)")
		_, err = admin.PublishSchema(ctx, s.ID, 1)
		require.NoError(t, err)
		tenant, err := admin.CreateTenant(ctx, "sec-json-depth-tenant-"+randSuffix(), s.ID, 1)
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = admin.DeleteTenant(context.Background(), tenant.ID)
			_ = admin.DeleteSchema(context.Background(), s.ID)
		})

		// SetField must return quickly — if the server hangs on JSON Schema
		// compilation, the 10-second context will time out and fail the test.
		valCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cfg := newConfigClient(conn)
		_ = cfg.Set(valCtx, tenant.ID, "data.payload", `{"ok":true}`)
		require.NoError(t, valCtx.Err(), "server must not hang on pathological JSON Schema compile")
	})
}

// buildDeepJSONSchema returns a valid JSON Schema string with the given
// nesting depth. depth=1 produces {"type":"string"}; each additional level
// wraps it in an object property.
func buildDeepJSONSchema(depth int) string {
	if depth <= 0 {
		return `{"type":"string"}`
	}
	return fmt.Sprintf(`{"type":"object","properties":{"a":%s}}`, buildDeepJSONSchema(depth-1))
}

// TestSecurity_AuditChainIntegrity: after legitimate config writes, the
// server-side VerifyChain RPC must find no breaks. Regression for decree#218.
//
// Uses the AuditService.VerifyChain RPC (server-side ordering + hashing)
// rather than the SDK client-side implementation to avoid sort instability
// when entries share the same microsecond timestamp.
//
// SQL-level tamper detection (UPDATE/DELETE on audit rows breaks the chain)
// is covered by the unit test in internal/audit/verify_test.go.
func TestSecurity_AuditChainIntegrity(t *testing.T) {
	fixture := bootstrapMatrixFixture(t, "sec-audit")
	conn := dial(t)
	cfg := newConfigClient(conn)
	auditSvc := pb.NewAuditServiceClient(conn)

	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-subject", "e2e-security-audit", "x-role", "superadmin")

	// Lay down several config writes to build up a multi-entry chain.
	require.NoError(t, cfg.Set(ctx, fixture.tenantID, "app.name", "alpha"))
	require.NoError(t, cfg.Set(ctx, fixture.tenantID, "app.name", "beta"))
	require.NoError(t, cfg.Set(ctx, fixture.tenantID, "app.name", "gamma"))

	resp, err := auditSvc.VerifyChain(ctx, &pb.VerifyChainRequest{TenantId: fixture.tenantID})
	require.NoError(t, err)
	assert.True(t, resp.GetOk(),
		"audit chain must be intact after legitimate writes; unexpected breaks: %v", resp.GetBreaks())
	assert.Greater(t, int(resp.GetTotal()), 0,
		"audit chain must contain at least one entry after config writes")
}

// TestSecurity_OversizeMetadataRejected: a request with an x-subject header
// exceeding 1024 bytes is rejected with codes.InvalidArgument before the
// handler runs. Regression for decree#219.
func TestSecurity_OversizeMetadataRejected(t *testing.T) {
	conn := dial(t)
	schemaSvc := pb.NewSchemaServiceClient(conn)

	// maxHeaderLen (auth/metadata.go) = 1024; send 1025 bytes.
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		"x-subject", strings.Repeat("x", 1025),
	))

	_, err := schemaSvc.GetSchema(ctx, &pb.GetSchemaRequest{
		Id: "00000000-0000-0000-0000-000000000000",
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err),
		"oversize x-subject header must be rejected with InvalidArgument")
}

// TestSecurity_ReflectionEnabled: with ENABLE_REFLECTION=1 (docker-compose),
// ServerReflectionInfo succeeds and lists registered services.
// Regression for decree#223.
//
// The "disabled by default" side is covered by
// internal/server/server_test.go:TestReflection_DisabledByDefault_ReturnsUnimplemented.
func TestSecurity_ReflectionEnabled(t *testing.T) {
	conn := dial(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Reflection is registered as a regular gRPC service and goes through the
	// same auth interceptor, so x-subject is required.
	ctx = metadata.AppendToOutgoingContext(ctx, "x-subject", "e2e-security-reflection", "x-role", "superadmin")

	client := reflpb.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflpb.ServerReflectionRequest{
		MessageRequest: &reflpb.ServerReflectionRequest_ListServices{ListServices: ""},
	}))
	resp, err := stream.Recv()
	require.NoError(t, err, "ServerReflectionInfo must succeed when ENABLE_REFLECTION=1")
	svcs := resp.GetListServicesResponse().GetService()
	assert.NotEmpty(t, svcs, "reflection must list at least one registered service")
}

// TestSecurity_ImportConfigSensitiveFieldRedacted: ImportConfig on a schema
// with sensitive fields must write [REDACTED] to the audit log and publish
// [REDACTED] in Subscribe events — plaintext must never appear on any read path.
// Regression for decree#416.
func TestSecurity_ImportConfigSensitiveFieldRedacted(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfgSvc := pb.NewConfigServiceClient(conn)
	ctx := context.Background()

	const secretValue = "import-s3cr3t-abc123"

	s, err := admin.CreateSchema(ctx, "sec-import-sensitive-"+randSuffix(), []adminclient.Field{
		{Path: "auth.token", Type: "FIELD_TYPE_STRING", Sensitive: true},
		{Path: "app.name", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)
	tenant, err := admin.CreateTenant(ctx, "sec-import-sensitive-tenant-"+randSuffix(), s.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = admin.DeleteTenant(context.Background(), tenant.ID)
		_ = admin.DeleteSchema(context.Background(), s.ID)
	})

	// Subscribe before the import so we capture the event.
	subCtx, subCancel := context.WithTimeout(ctx, 10*time.Second)
	defer subCancel()
	subCtx = metadata.AppendToOutgoingContext(subCtx, "x-subject", "e2e-security-import-sensitive", "x-role", "superadmin")
	stream, err := cfgSvc.Subscribe(subCtx, &pb.SubscribeRequest{
		TenantId:   tenant.ID,
		FieldPaths: []string{"auth.token"},
	})
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Import YAML containing a sensitive field.
	yamlContent := []byte("spec_version: v1\nvalues:\n  auth.token:\n    value: " + secretValue + "\n  app.name:\n    value: myapp\n")
	_, err = admin.ImportConfig(ctx, tenant.ID, yamlContent, "import sensitive test")
	require.NoError(t, err)

	// --- Subscribe event must not carry plaintext ---
	event, err := stream.Recv()
	require.NoError(t, err)
	newVal := event.GetChange().GetNewValue().GetStringValue()
	assert.Equal(t, "[REDACTED]", newVal, "subscribe event must not carry plaintext sensitive value after ImportConfig")
	assert.NotEqual(t, secretValue, newVal)

	// --- QueryWriteLog must not carry plaintext ---
	entries, err := admin.QueryWriteLog(ctx, adminclient.WithAuditTenant(tenant.ID))
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		assert.NotEqual(t, secretValue, e.NewValue,
			"audit log new_value must not carry plaintext sensitive value after ImportConfig")
		assert.NotEqual(t, secretValue, e.OldValue,
			"audit log old_value must not carry plaintext sensitive value after ImportConfig")
	}
}
