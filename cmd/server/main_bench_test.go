package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/audit"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/cache"
	"github.com/opendecree/decree/internal/config"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/schema"
	"github.com/opendecree/decree/internal/server"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/telemetry"
	"github.com/opendecree/decree/internal/validation"
)

// BenchmarkServerColdStart measures time from the start of server
// initialization (in-memory backend, all three services) to the moment the
// first gRPC health-check RPC returns SERVING.
func BenchmarkServerColdStart(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()
	noopCfg := telemetry.Config{}

	for b.Loop() {
		// --- Initialize in-memory stores (mirrors run() memory-backend path) ---
		memConfig := config.NewMemoryStore()
		memSchema := schema.NewMemoryStore()
		memPubSub := pubsub.NewMemoryPubSub()
		validatorStore := &validation.SchemaStoreAdapter{
			GetTenantByIDFn: memSchema.GetTenantByID,
			GetSchemaVersionFn: func(ctx context.Context, schemaID string, version int32) (domain.SchemaVersion, error) {
				return memSchema.GetSchemaVersion(ctx, schema.GetSchemaVersionParams{SchemaID: schemaID, Version: version})
			},
			GetSchemaFieldsFn: memSchema.GetSchemaFields,
		}
		validatorFactory := validation.NewValidatorFactory(validatorStore)
		nullLog := slog.New(slog.NewTextHandler(io.Discard, nil))

		// --- Create gRPC server (binds port) ---
		srv, err := server.New("0", auth.NewMetadataInterceptor(nil),
			server.WithEnableServices([]string{"schema", "config", "audit"}),
			server.WithLogger(nullLog),
			server.WithInsecure(),
		)
		if err != nil {
			b.Fatal(err)
		}
		addr := srv.Addr().String()

		// --- Register services ---
		pb.RegisterSchemaServiceServer(srv.GRPCServer(), schema.NewService(memSchema,
			schema.WithLogger(nullLog),
			schema.WithMetrics(telemetry.NewSchemaMetrics(noopCfg)),
			schema.WithValidators(validatorFactory),
		))
		configSvc := config.NewService(memConfig, cache.NewMemoryCache(context.Background(), 0), memPubSub, memPubSub,
			config.WithLogger(nullLog),
			config.WithCacheMetrics(telemetry.NewCacheMetrics(noopCfg)),
			config.WithMetrics(telemetry.NewConfigMetrics(noopCfg)),
			config.WithValidators(validatorFactory),
		)
		pb.RegisterConfigServiceServer(srv.GRPCServer(), configSvc)
		pb.RegisterAuditServiceServer(srv.GRPCServer(), audit.NewService(audit.NewMemoryStore(), nullLog, nil))
		srv.SetServiceHealthy("centralconfig.v1.SchemaService")
		srv.SetServiceHealthy("centralconfig.v1.ConfigService")
		srv.SetServiceHealthy("centralconfig.v1.AuditService")

		// --- Start serving ---
		serveCtx, serveCancel := context.WithCancel(ctx)
		go func() { _ = srv.Serve(serveCtx) }()

		// --- First RPC: wait until SERVING ---
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			serveCancel()
			srv.GracefulStop(ctx)
			b.Fatal(err)
		}
		_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx,
			&grpc_health_v1.HealthCheckRequest{},
			grpc.WaitForReady(true),
		)
		_ = conn.Close()
		serveCancel()
		srv.GracefulStop(ctx)
		_ = memPubSub.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}
