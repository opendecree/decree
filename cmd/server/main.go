package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/exaring/otelpgx"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/internal/audit"
	"github.com/zeevdr/central-config-service/internal/auth"
	"github.com/zeevdr/central-config-service/internal/cache"
	"github.com/zeevdr/central-config-service/internal/config"
	"github.com/zeevdr/central-config-service/internal/pubsub"
	"github.com/zeevdr/central-config-service/internal/schema"
	"github.com/zeevdr/central-config-service/internal/server"
	"github.com/zeevdr/central-config-service/internal/storage"
	"github.com/zeevdr/central-config-service/internal/telemetry"
	"github.com/zeevdr/central-config-service/internal/validation"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg := loadConfig()
	otelCfg := telemetry.ConfigFromEnv()

	// Logger — wrap with trace correlation if OTel is enabled.
	logger := newLogger(cfg.LogLevel, otelCfg.Enabled)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Telemetry.
	otelShutdown, err := telemetry.Init(ctx, otelCfg)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize telemetry", "error", err)
		return 1
	}
	defer func() { _ = otelShutdown(ctx) }()
	if otelCfg.Enabled {
		logger.InfoContext(ctx, "telemetry enabled",
			"traces_grpc", otelCfg.TracesGRPC,
			"traces_db", otelCfg.TracesDB,
			"traces_redis", otelCfg.TracesRedis,
			"metrics_grpc", otelCfg.MetricsGRPC,
			"metrics_db_pool", otelCfg.MetricsDBPool,
			"metrics_cache", otelCfg.MetricsCache,
			"metrics_config", otelCfg.MetricsConfig,
			"metrics_schema", otelCfg.MetricsSchema,
		)
	}

	// Database.
	var dbOpts []storage.Option
	if otelCfg.TracesDB {
		dbOpts = append(dbOpts, storage.WithTracer(otelpgx.NewTracer()))
	}
	db, err := storage.NewDB(ctx, cfg.DBWriteURL, cfg.DBReadURL, dbOpts...)
	if err != nil {
		logger.ErrorContext(ctx, "failed to connect to database", "error", err)
		return 1
	}
	defer db.Close()
	logger.InfoContext(ctx, "connected to database")

	// Redis.
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.ErrorContext(ctx, "failed to parse redis url", "error", err)
		return 1
	}
	redisClient := redis.NewClient(redisOpts)
	defer func() { _ = redisClient.Close() }()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.ErrorContext(ctx, "failed to connect to redis", "error", err)
		return 1
	}
	if otelCfg.TracesRedis {
		if err := redisotel.InstrumentTracing(redisClient); err != nil {
			logger.ErrorContext(ctx, "failed to instrument redis tracing", "error", err)
			return 1
		}
	}
	logger.InfoContext(ctx, "connected to redis")

	// Cache and pub/sub.
	configCache := cache.NewRedisCache(redisClient)
	publisher := pubsub.NewRedisPublisher(redisClient)
	subscriber := pubsub.NewRedisSubscriber(redisClient, logger)
	defer func() { _ = publisher.Close() }()
	defer func() { _ = subscriber.Close() }()

	// Auth interceptor.
	var authInterceptor server.GRPCInterceptor
	if cfg.JWTJWKSURL != "" {
		jwtInterceptor, jwtErr := auth.NewInterceptor(ctx, cfg.JWTJWKSURL, cfg.JWTIssuer, logger)
		if jwtErr != nil {
			logger.ErrorContext(ctx, "failed to create auth interceptor", "error", jwtErr)
			return 1
		}
		defer jwtInterceptor.Close()
		authInterceptor = jwtInterceptor
		logger.InfoContext(ctx, "JWT auth enabled", "jwks_url", cfg.JWTJWKSURL)
	} else {
		authInterceptor = auth.NewMetadataInterceptor()
		logger.InfoContext(ctx, "metadata auth enabled — pass x-subject, x-role, x-tenant-id headers")
	}

	// gRPC server with optional OTel stats handler.
	var extraOpts []grpc.ServerOption
	if otelCfg.TracesGRPC {
		extraOpts = append(extraOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}

	srv, err := server.New(server.Config{
		GRPCPort:        cfg.GRPCPort,
		EnableServices:  cfg.EnableServices,
		Logger:          logger,
		AuthInterceptor: authInterceptor,
		ExtraOptions:    extraOpts,
	})
	if err != nil {
		logger.ErrorContext(ctx, "failed to create server", "error", err)
		return 1
	}

	// Metrics (nil when disabled — all metric types handle nil receiver).
	cacheMetrics := telemetry.NewCacheMetrics(otelCfg)
	configMetrics := telemetry.NewConfigMetrics(otelCfg)
	schemaMetrics := telemetry.NewSchemaMetrics(otelCfg)
	telemetry.StartDBPoolMetrics(ctx, otelCfg, db.WritePool, db.ReadPool)

	// Validator factory (shared between schema + config services).
	configStore := config.NewPGStore(db.WritePool, db.ReadPool)
	validatorFactory := validation.NewValidatorFactory(configStore)

	// Register services.
	if srv.IsServiceEnabled("schema") {
		schemaStore := schema.NewPGStore(db.WritePool, db.ReadPool)
		schemaSvc := schema.NewService(schemaStore, logger, schemaMetrics, validatorFactory.Cache())
		pb.RegisterSchemaServiceServer(srv.GRPCServer(), schemaSvc)
		srv.SetServiceHealthy("centralconfig.v1.SchemaService")
		logger.InfoContext(ctx, "schema service enabled")
	}
	if srv.IsServiceEnabled("config") {
		configSvc := config.NewService(configStore, configCache, publisher, subscriber, logger, cacheMetrics, configMetrics, validatorFactory)
		pb.RegisterConfigServiceServer(srv.GRPCServer(), configSvc)
		srv.SetServiceHealthy("centralconfig.v1.ConfigService")
		logger.InfoContext(ctx, "config service enabled")
	}
	if srv.IsServiceEnabled("audit") {
		auditStore := audit.NewPGStore(db.WritePool, db.ReadPool)
		auditSvc := audit.NewService(auditStore, logger)
		pb.RegisterAuditServiceServer(srv.GRPCServer(), auditSvc)
		srv.SetServiceHealthy("centralconfig.v1.AuditService")
		logger.InfoContext(ctx, "audit service enabled")
	}

	// Start server in background.
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx)
	}()

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.InfoContext(ctx, "received signal, shutting down", "signal", sig)
	case err := <-errCh:
		logger.ErrorContext(ctx, "server error", "error", err)
	}

	cancel()
	srv.GracefulStop(ctx)
	logger.InfoContext(ctx, "central-config-service stopped")
	return 0
}

type serverConfig struct {
	GRPCPort       string
	DBWriteURL     string
	DBReadURL      string
	RedisURL       string
	EnableServices []string
	JWTIssuer      string
	JWTJWKSURL     string
	LogLevel       string
}

func loadConfig() serverConfig {
	enableServices := getEnv("ENABLE_SERVICES", "schema,config,audit")
	dbWriteURL := getEnv("DB_WRITE_URL", "")
	dbReadURL := getEnv("DB_READ_URL", dbWriteURL)

	return serverConfig{
		GRPCPort:       getEnv("GRPC_PORT", "9090"),
		DBWriteURL:     dbWriteURL,
		DBReadURL:      dbReadURL,
		RedisURL:       getEnv("REDIS_URL", ""),
		EnableServices: parseServices(enableServices),
		JWTIssuer:      getEnv("JWT_ISSUER", ""),
		JWTJWKSURL:     getEnv("JWT_JWKS_URL", ""),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseServices(s string) []string {
	var services []string
	for _, svc := range strings.Split(s, ",") {
		svc = strings.TrimSpace(svc)
		if svc != "" {
			switch svc {
			case "schema", "config", "audit":
				services = append(services, svc)
			default:
				slog.Error("unknown service", "service", svc)
				os.Exit(1)
			}
		}
	}
	if len(services) == 0 {
		slog.Error("no services enabled")
		os.Exit(1)
	}
	return services
}

func newLogger(level string, traceCorrelation bool) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	if traceCorrelation {
		handler = telemetry.NewLogHandler(handler)
	}
	return slog.New(handler)
}
