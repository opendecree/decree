package server

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/audit"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/cache"
	"github.com/opendecree/decree/internal/config"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/schema"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/telemetry"
	"github.com/opendecree/decree/internal/validation"
)

// TestMemoryBackend_Integration starts a full server with in-memory storage
// and verifies the core schema→tenant→config flow works end-to-end.
func TestMemoryBackend_Integration(t *testing.T) {
	ctx := context.Background()
	authCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-subject", "integration-test"))

	// Create server.
	srv, err := New("0", auth.NewMetadataInterceptor(nil),
		WithEnableServices([]string{"schema", "config", "audit"}),
		WithLogger(slog.Default()),
		WithInsecure(),
	)
	require.NoError(t, err)

	// Wire in-memory stores.
	memConfig := config.NewMemoryStore()
	memSchema := schema.NewMemoryStore()
	memPubSub := pubsub.NewMemoryPubSub()

	// Validator needs tenant/schema data from the schema store.
	validatorStore := &validation.SchemaStoreAdapter{
		GetTenantByIDFn: memSchema.GetTenantByID,
		GetSchemaVersionFn: func(ctx context.Context, schemaID string, version int32) (domain.SchemaVersion, error) {
			return memSchema.GetSchemaVersion(ctx, schema.GetSchemaVersionParams{SchemaID: schemaID, Version: version})
		},
		GetSchemaFieldsFn: memSchema.GetSchemaFields,
	}
	validatorFactory := validation.NewValidatorFactory(validatorStore)

	schemaSvc := schema.NewService(memSchema,
		schema.WithLogger(slog.Default()),
		schema.WithMetrics(telemetry.NewSchemaMetrics(telemetry.Config{})),
		schema.WithValidators(validatorFactory),
	)
	pb.RegisterSchemaServiceServer(srv.GRPCServer(), schemaSvc)

	configSvc := config.NewService(memConfig, cache.NewMemoryCache(0), memPubSub, memPubSub,
		config.WithLogger(slog.Default()),
		config.WithCacheMetrics(telemetry.NewCacheMetrics(telemetry.Config{})),
		config.WithMetrics(telemetry.NewConfigMetrics(telemetry.Config{})),
		config.WithValidators(validatorFactory),
	)
	pb.RegisterConfigServiceServer(srv.GRPCServer(), configSvc)

	auditSvc := audit.NewService(audit.NewMemoryStore(), slog.Default(), nil)
	pb.RegisterAuditServiceServer(srv.GRPCServer(), auditSvc)

	// Start server.
	go func() { _ = srv.Serve(ctx) }()
	t.Cleanup(func() { srv.GracefulStop(ctx) })
	time.Sleep(50 * time.Millisecond)

	// Connect.
	addr := srv.listener.Addr().String()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	schemaClient := pb.NewSchemaServiceClient(conn)
	configClient := pb.NewConfigServiceClient(conn)

	// 1. Create schema.
	createResp, err := schemaClient.CreateSchema(authCtx, &pb.CreateSchemaRequest{
		Name: "test-payments",
		Fields: []*pb.SchemaField{
			{Path: "fee", Type: pb.FieldType_FIELD_TYPE_NUMBER, Nullable: true},
			{Path: "enabled", Type: pb.FieldType_FIELD_TYPE_BOOL},
		},
	})
	require.NoError(t, err)
	schemaID := createResp.Schema.Id
	assert.Equal(t, "test-payments", createResp.Schema.Name)
	assert.Equal(t, int32(1), createResp.Schema.Version)

	// 2. Publish schema.
	_, err = schemaClient.PublishSchema(authCtx, &pb.PublishSchemaRequest{Id: schemaID, Version: 1})
	require.NoError(t, err)

	// 3. Create tenant.
	tenantResp, err := schemaClient.CreateTenant(authCtx, &pb.CreateTenantRequest{
		Name: "acme", SchemaId: schemaID, SchemaVersion: 1,
	})
	require.NoError(t, err)
	tenantID := tenantResp.Tenant.Id

	// Seed memConfig with tenant and schema data so getSensitiveFieldSet can
	// resolve tenant→schema info. In production a single Postgres DB serves
	// both; separate in-memory stores require explicit bridging.
	{
		tenant2, err2 := memSchema.GetTenantByID(ctx, tenantID)
		require.NoError(t, err2)
		memConfig.SetTenant(tenant2)

		sv2, err2 := memSchema.GetSchemaVersion(ctx, schema.GetSchemaVersionParams{SchemaID: schemaID, Version: 1})
		require.NoError(t, err2)
		memConfig.SetSchemaVersion(sv2)

		fields2, err2 := memSchema.GetSchemaFields(ctx, sv2.ID)
		require.NoError(t, err2)
		memConfig.SetSchemaFields(sv2.ID, fields2)
	}

	// 4. Set config value.
	_, err = configClient.SetField(authCtx, &pb.SetFieldRequest{
		TenantId:  tenantID,
		FieldPath: "fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0.025}},
	})
	require.NoError(t, err)

	// 5. Read config value.
	getResp, err := configClient.GetField(authCtx, &pb.GetFieldRequest{
		TenantId: tenantID, FieldPath: "fee",
	})
	require.NoError(t, err)
	assert.Equal(t, 0.025, getResp.Value.GetValue().GetNumberValue())

	// 6. List versions.
	versionsResp, err := configClient.ListVersions(authCtx, &pb.ListVersionsRequest{TenantId: tenantID})
	require.NoError(t, err)
	assert.Len(t, versionsResp.Versions, 1)

	// 7. Cleanup.
	_, err = schemaClient.DeleteTenant(authCtx, &pb.DeleteTenantRequest{Id: tenantID})
	require.NoError(t, err)
	_, err = schemaClient.DeleteSchema(authCtx, &pb.DeleteSchemaRequest{Id: schemaID})
	require.NoError(t, err)
}
