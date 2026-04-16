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

	// Create server.
	srv, err := New(Config{
		GRPCPort:        "0",
		EnableServices:  []string{"schema", "config", "audit"},
		Logger:          slog.Default(),
		AuthInterceptor: &noopInterceptor{},
	})
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

	schemaSvc := schema.NewService(memSchema, slog.Default(), telemetry.NewSchemaMetrics(telemetry.Config{}), validatorFactory.Cache())
	pb.RegisterSchemaServiceServer(srv.GRPCServer(), schemaSvc)

	configSvc := config.NewService(config.ServiceConfig{
		Store:        memConfig,
		Cache:        cache.NewMemoryCache(0),
		Publisher:    memPubSub,
		Subscriber:   memPubSub,
		Logger:       slog.Default(),
		CacheMetrics: telemetry.NewCacheMetrics(telemetry.Config{}),
		Metrics:      telemetry.NewConfigMetrics(telemetry.Config{}),
		Validators:   validatorFactory,
	})
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
	createResp, err := schemaClient.CreateSchema(ctx, &pb.CreateSchemaRequest{
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
	_, err = schemaClient.PublishSchema(ctx, &pb.PublishSchemaRequest{Id: schemaID, Version: 1})
	require.NoError(t, err)

	// 3. Create tenant.
	tenantResp, err := schemaClient.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name: "acme", SchemaId: schemaID, SchemaVersion: 1,
	})
	require.NoError(t, err)
	tenantID := tenantResp.Tenant.Id

	// 4. Set config value.
	authCtx := metadata.AppendToOutgoingContext(ctx, "x-subject", "test-user")
	_, err = configClient.SetField(authCtx, &pb.SetFieldRequest{
		TenantId:  tenantID,
		FieldPath: "fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0.025}},
	})
	require.NoError(t, err)

	// 5. Read config value.
	getResp, err := configClient.GetField(ctx, &pb.GetFieldRequest{
		TenantId: tenantID, FieldPath: "fee",
	})
	require.NoError(t, err)
	assert.Equal(t, 0.025, getResp.Value.GetValue().GetNumberValue())

	// 6. List versions.
	versionsResp, err := configClient.ListVersions(ctx, &pb.ListVersionsRequest{TenantId: tenantID})
	require.NoError(t, err)
	assert.Len(t, versionsResp.Versions, 1)

	// 7. Cleanup.
	_, err = schemaClient.DeleteTenant(ctx, &pb.DeleteTenantRequest{Id: tenantID})
	require.NoError(t, err)
	_, err = schemaClient.DeleteSchema(ctx, &pb.DeleteSchemaRequest{Id: schemaID})
	require.NoError(t, err)
}
