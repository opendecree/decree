package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/audit"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/cache"
	"github.com/opendecree/decree/internal/config"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/ratelimit"
	"github.com/opendecree/decree/internal/schema"
	"github.com/opendecree/decree/internal/server"
	"github.com/opendecree/decree/internal/storage"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/telemetry"
	"github.com/opendecree/decree/internal/validation"
	"github.com/opendecree/decree/internal/version"

	"golang.org/x/time/rate"
)

//go:embed openapi.json
var openAPISpec []byte

func main() {
	os.Exit(run())
}

func run() int {
	cfg := loadConfig()
	otelCfg := telemetry.ConfigFromEnv()

	logger := newLogger(cfg.LogLevel)
	logger.Info("starting decree", "version", version.Version, "commit", version.Commit)

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

	// Storage backend.
	var (
		configStore    config.Store
		schemaStoreVal schema.Store
		auditStoreVal  audit.Store
		configCache    cache.ConfigCache
		publisher      pubsub.Publisher
		subscriber     pubsub.Subscriber
		validatorStore validation.Store
	)

	if cfg.StorageBackend == "memory" {
		logger.InfoContext(ctx, "using in-memory storage (no PostgreSQL or Redis required)")
		memConfig := config.NewMemoryStore()
		memSchema := schema.NewMemoryStore()
		configStore = memConfig
		schemaStoreVal = memSchema
		auditStoreVal = audit.NewMemoryStore()
		configCache = cache.NewMemoryCache(0)
		memPubSub := pubsub.NewMemoryPubSub()
		publisher = memPubSub
		subscriber = memPubSub
		defer func() { _ = publisher.Close() }()
		// Validator needs tenant/schema data — use schema store via adapter.
		validatorStore = &validation.SchemaStoreAdapter{
			GetTenantByIDFn: memSchema.GetTenantByID,
			GetSchemaVersionFn: func(ctx context.Context, schemaID string, version int32) (domain.SchemaVersion, error) {
				return memSchema.GetSchemaVersion(ctx, schema.GetSchemaVersionParams{SchemaID: schemaID, Version: version})
			},
			GetSchemaFieldsFn: memSchema.GetSchemaFields,
		}
	} else {
		// Parse Redis URL up-front so a malformed URL fails before we spawn
		// any connect goroutines (also avoids a torn-down DB pool on URL error).
		redisOpts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.ErrorContext(ctx, "failed to parse redis url", "error", err)
			return 1
		}

		// Connect to PostgreSQL and Redis in parallel — independent backends,
		// each blocking on network I/O. First failure cancels the other.
		var (
			db          *storage.DB
			redisClient *redis.Client
		)
		initG, initCtx := errgroup.WithContext(ctx)
		initG.Go(func() error {
			var dbOpts []storage.Option
			if otelCfg.TracesDB {
				dbOpts = append(dbOpts, storage.WithTracer(otelpgx.NewTracer()))
			}
			d, err := storage.NewDB(initCtx, cfg.DBWriteURL, cfg.DBReadURL, dbOpts...)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			db = d
			return nil
		})
		initG.Go(func() error {
			c := redis.NewClient(redisOpts)
			if err := c.Ping(initCtx).Err(); err != nil {
				_ = c.Close()
				return fmt.Errorf("connect to redis: %w", err)
			}
			if otelCfg.TracesRedis {
				if err := redisotel.InstrumentTracing(c); err != nil {
					_ = c.Close()
					return fmt.Errorf("instrument redis tracing: %w", err)
				}
			}
			redisClient = c
			return nil
		})
		if err := initG.Wait(); err != nil {
			// Either backend failed; close the survivor (if any) and bail.
			if db != nil {
				db.Close()
			}
			if redisClient != nil {
				_ = redisClient.Close()
			}
			logger.ErrorContext(ctx, "failed to initialize backends", "error", err)
			return 1
		}
		defer db.Close()
		defer func() { _ = redisClient.Close() }()
		logger.InfoContext(ctx, "connected to database")
		logger.InfoContext(ctx, "connected to redis")

		configStore = config.NewPGStore(db.WritePool, db.ReadPool)
		schemaStoreVal = schema.NewPGStore(db.WritePool, db.ReadPool)
		auditStoreVal = audit.NewPGStore(db.WritePool, db.ReadPool)
		configCache = cache.NewRedisCache(redisClient)
		publisher = pubsub.NewRedisPublisher(redisClient)
		subscriber = pubsub.NewRedisSubscriber(redisClient, logger)
		defer func() { _ = publisher.Close() }()
		defer func() { _ = subscriber.Close() }()
		validatorStore = configStore

		telemetry.StartDBPoolMetrics(ctx, otelCfg, db.WritePool, db.ReadPool)
	}

	// Auth interceptor.
	var authInterceptor server.GRPCInterceptor
	if cfg.JWTJWKSURL != "" {
		jwtInterceptor, jwtErr := auth.NewInterceptor(ctx, cfg.JWTJWKSURL,
			auth.WithIssuer(cfg.JWTIssuer),
			auth.WithLogger(logger),
		)
		if jwtErr != nil {
			logger.ErrorContext(ctx, "failed to create auth interceptor", "error", jwtErr)
			return 1
		}
		defer jwtInterceptor.Close()
		authInterceptor = jwtInterceptor
		logger.InfoContext(ctx, "JWT auth enabled", "jwks_url", cfg.JWTJWKSURL)
	} else {
		authInterceptor = auth.NewMetadataInterceptor(tenantResolver(schemaStoreVal), auth.WithMetadataLogger(logger))
		logger.InfoContext(ctx, "metadata auth enabled — pass x-subject, x-role, x-tenant-id headers")
	}

	// gRPC server with optional OTel stats handler.
	var extraOpts []grpc.ServerOption
	if otelCfg.TracesGRPC {
		extraOpts = append(extraOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}

	var serverTLS *server.TLSConfig
	if !cfg.InsecureListen {
		serverTLS = &server.TLSConfig{
			CertFile:     cfg.TLSCertFile,
			KeyFile:      cfg.TLSKeyFile,
			ClientCAFile: cfg.TLSClientCAFile,
		}
	} else {
		logger.WarnContext(ctx, "INSECURE_LISTEN=1 set — gRPC server will accept plaintext connections (local dev only)")
	}

	// Rate limiter (nil when disabled; nil WithRateLimiter is a no-op).
	rlMetrics := telemetry.NewRateLimitMetrics(otelCfg)
	var rlInterceptor *ratelimit.Interceptor
	if cfg.RateLimitEnabled {
		rlCfg := ratelimit.Config{
			Anonymous:     ratelimit.NewInProcess(rate.Limit(cfg.RateLimitAnonRPS), cfg.RateLimitBurst),
			Authenticated: ratelimit.NewInProcess(rate.Limit(cfg.RateLimitAuthedRPS), cfg.RateLimitBurst),
		}
		if cfg.RateLimitSuperAdminRPS > 0 {
			rlCfg.SuperAdmin = ratelimit.NewInProcess(rate.Limit(cfg.RateLimitSuperAdminRPS), cfg.RateLimitBurst)
		}
		rlOpts := []ratelimit.Option{ratelimit.WithInterceptorLogger(logger)}
		if counter, ok := rlMetrics.Counter(); ok {
			rlOpts = append(rlOpts, ratelimit.WithRejectedCounter(counter))
		}
		rlInterceptor = ratelimit.New(rlCfg, rlOpts...)
		logger.InfoContext(ctx, "rate limiting enabled",
			"anon_rps", cfg.RateLimitAnonRPS,
			"authed_rps", cfg.RateLimitAuthedRPS,
			"superadmin_rps", cfg.RateLimitSuperAdminRPS,
			"burst", cfg.RateLimitBurst,
		)
	}

	srvOpts := []server.Option{
		server.WithEnableServices(cfg.EnableServices),
		server.WithLogger(logger),
		server.WithGRPCServerOptions(extraOpts...),
		server.WithMaxRecvMsgBytes(cfg.GRPCMaxRecvMsgBytes),
		server.WithMaxSendMsgBytes(cfg.GRPCMaxSendMsgBytes),
	}
	if cfg.InsecureListen {
		srvOpts = append(srvOpts, server.WithInsecure())
	} else {
		srvOpts = append(srvOpts, server.WithTLS(serverTLS))
	}
	if rlInterceptor != nil {
		srvOpts = append(srvOpts, server.WithRateLimiter(rlInterceptor))
	}
	srv, err := server.New(cfg.GRPCPort, authInterceptor, srvOpts...)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create server", "error", err)
		return 1
	}

	// Metrics (nil when disabled — all metric types handle nil receiver).
	cacheMetrics := telemetry.NewCacheMetrics(otelCfg)
	configMetrics := telemetry.NewConfigMetrics(otelCfg)
	schemaMetrics := telemetry.NewSchemaMetrics(otelCfg)

	// Validator factory (shared between schema + config services).
	validatorFactory := validation.NewValidatorFactory(validatorStore,
		validation.WithLimits(validation.Limits{
			CompileTimeout: cfg.SchemaCompileTimeout,
			MaxDepth:       cfg.SchemaMaxRefDepth,
		}),
	)

	// Usage recorder — async batched read tracking for audit stats.
	var recorder *audit.UsageRecorder
	if cfg.UsageTrackingEnabled {
		recorder = audit.NewUsageRecorder(auditStoreVal,
			audit.WithFlushInterval(cfg.UsageFlushInterval),
			audit.WithLogger(logger),
		)
		go recorder.Start(ctx)
		logger.InfoContext(ctx, "usage tracking enabled", "flush_interval", cfg.UsageFlushInterval)
	} else {
		logger.InfoContext(ctx, "usage tracking disabled")
	}

	// Server info service — always registered, no auth.
	serverInfoSvc := server.NewServerService(server.Features{
		Schema:        srv.IsServiceEnabled("schema"),
		Config:        srv.IsServiceEnabled("config"),
		Audit:         srv.IsServiceEnabled("audit"),
		UsageTracking: cfg.UsageTrackingEnabled,
		JWTAuth:       cfg.JWTJWKSURL != "",
		HTTPGateway:   cfg.HTTPPort != "",
	})
	pb.RegisterServerServiceServer(srv.GRPCServer(), serverInfoSvc)

	// Register services.
	if srv.IsServiceEnabled("schema") {
		schemaSvc := schema.NewService(schemaStoreVal,
			schema.WithLogger(logger),
			schema.WithMetrics(schemaMetrics),
			schema.WithValidators(validatorFactory),
			schema.WithLimits(schema.Limits{
				MaxFields:   cfg.SchemaMaxFields,
				MaxDocBytes: cfg.SchemaMaxDocBytes,
			}),
		)
		pb.RegisterSchemaServiceServer(srv.GRPCServer(), schemaSvc)
		srv.SetServiceHealthy("centralconfig.v1.SchemaService")
		logger.InfoContext(ctx, "schema service enabled")
	}
	if srv.IsServiceEnabled("config") {
		configSvc := config.NewService(configStore, configCache, publisher, subscriber,
			config.WithLogger(logger),
			config.WithCacheMetrics(cacheMetrics),
			config.WithMetrics(configMetrics),
			config.WithValidators(validatorFactory),
			config.WithRecorder(recorder),
		)
		pb.RegisterConfigServiceServer(srv.GRPCServer(), configSvc)
		srv.SetServiceHealthy("centralconfig.v1.ConfigService")
		logger.InfoContext(ctx, "config service enabled")
	}
	if srv.IsServiceEnabled("audit") {
		auditSvc := audit.NewService(auditStoreVal, logger, func(ctx context.Context, idOrName string) (string, error) {
			return tenantResolver(schemaStoreVal)(ctx, idOrName)
		})
		pb.RegisterAuditServiceServer(srv.GRPCServer(), auditSvc)
		srv.SetServiceHealthy("centralconfig.v1.AuditService")
		logger.InfoContext(ctx, "audit service enabled")
	}

	// Optional HTTP gateway (REST/JSON proxy to gRPC).
	var gw *server.Gateway
	if cfg.HTTPPort != "" {
		var gwTLS *server.GatewayTLSConfig
		if !cfg.InsecureListen {
			gwTLS = &server.GatewayTLSConfig{
				CAFile:         cfg.TLSGatewayCAFile,
				ServerName:     cfg.TLSGatewayServerName,
				ClientCertFile: cfg.TLSGatewayClientCertFile,
				ClientKeyFile:  cfg.TLSGatewayClientKeyFile,
			}
		}

		gwOpts := []server.GatewayOption{
			server.WithGatewayLogger(logger),
			server.WithOpenAPISpec(openAPISpec),
			server.WithGatewayMaxRecvMsgBytes(cfg.GRPCMaxRecvMsgBytes),
			server.WithGatewayMaxSendMsgBytes(cfg.GRPCMaxSendMsgBytes),
		}
		if cfg.InsecureListen {
			gwOpts = append(gwOpts, server.WithGatewayInsecure())
		} else {
			gwOpts = append(gwOpts, server.WithGatewayTLS(gwTLS))
		}
		gw, err = server.NewGateway(ctx, cfg.HTTPPort, fmt.Sprintf("localhost:%s", cfg.GRPCPort), gwOpts...)
		if err != nil {
			logger.ErrorContext(ctx, "failed to create HTTP gateway", "error", err)
			return 1
		}
	}

	// Run gRPC + (optional) HTTP gateway under errgroup. First failure cancels
	// the group context; both serve loops are joined deterministically via
	// g.Wait below — no manual errCh / send-on-closed races.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return srv.Serve(gctx) })
	if gw != nil {
		g.Go(func() error { return gw.Serve(gctx) })
	}

	// Wait for either a shutdown signal or an early serve failure.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		logger.InfoContext(ctx, "received signal, shutting down", "signal", sig)
	case <-gctx.Done():
		// A serve goroutine returned. The actual error is collected from g.Wait below.
	}

	// Trigger graceful shutdown. srv.Serve / gw.Serve do not unblock on ctx
	// cancellation — they need explicit Stop / Shutdown.
	cancel()
	recorder.Stop() // nil-safe; waits for final flush
	if gw != nil {
		gw.Shutdown(ctx)
	}
	srv.GracefulStop(ctx)

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.ErrorContext(ctx, "server error", "error", err)
		return 1
	}
	logger.InfoContext(ctx, "decree stopped")
	return 0
}

type serverConfig struct {
	GRPCPort                 string
	HTTPPort                 string
	StorageBackend           string
	DBWriteURL               string
	DBReadURL                string
	RedisURL                 string
	EnableServices           []string
	JWTIssuer                string
	JWTJWKSURL               string
	LogLevel                 string
	UsageTrackingEnabled     bool
	UsageFlushInterval       time.Duration
	GRPCMaxRecvMsgBytes      int
	GRPCMaxSendMsgBytes      int
	SchemaMaxFields          int
	SchemaMaxDocBytes        int
	SchemaCompileTimeout     time.Duration
	SchemaMaxRefDepth        int
	InsecureListen           bool
	TLSCertFile              string
	TLSKeyFile               string
	TLSClientCAFile          string
	TLSGatewayCAFile         string
	TLSGatewayServerName     string
	TLSGatewayClientCertFile string
	TLSGatewayClientKeyFile  string
	RateLimitEnabled         bool
	RateLimitAnonRPS         float64
	RateLimitAuthedRPS       float64
	RateLimitSuperAdminRPS   float64 // 0 = unlimited
	RateLimitBurst           int
}

// tenantResolver creates an auth.TenantResolver from a schema store.
// Resolves tenant name slugs to UUIDs so x-tenant-id headers accept both.
func tenantResolver(store schema.Store) auth.TenantResolver {
	return func(ctx context.Context, idOrName string) (string, error) {
		if domain.IsUUID(idOrName) {
			return idOrName, nil
		}
		tenant, err := store.GetTenantByName(ctx, idOrName)
		if err != nil {
			return "", err
		}
		return tenant.ID, nil
	}
}

func loadConfig() serverConfig {
	enableServices := getEnv("ENABLE_SERVICES", "schema,config,audit")
	dbWriteURL := getEnv("DB_WRITE_URL", "")
	dbReadURL := getEnv("DB_READ_URL", dbWriteURL)

	flushInterval := 30 * time.Second
	if v := getEnv("USAGE_FLUSH_INTERVAL", ""); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			flushInterval = d
		}
	}

	compileTimeout := 5 * time.Second
	if v := getEnv("SCHEMA_COMPILE_TIMEOUT", ""); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			slog.Error("invalid duration env var", "key", "SCHEMA_COMPILE_TIMEOUT", "value", v, "error", err)
			os.Exit(1)
		}
		compileTimeout = d
	}

	return serverConfig{
		GRPCPort:                 getEnv("GRPC_PORT", "9090"),
		HTTPPort:                 getEnv("HTTP_PORT", ""),
		StorageBackend:           getEnv("STORAGE_BACKEND", "postgres"),
		DBWriteURL:               dbWriteURL,
		DBReadURL:                dbReadURL,
		RedisURL:                 getEnv("REDIS_URL", ""),
		EnableServices:           parseServices(enableServices),
		JWTIssuer:                getEnv("JWT_ISSUER", ""),
		JWTJWKSURL:               getEnv("JWT_JWKS_URL", ""),
		LogLevel:                 getEnv("LOG_LEVEL", "info"),
		UsageTrackingEnabled:     getEnv("USAGE_TRACKING_ENABLED", "true") != "false",
		UsageFlushInterval:       flushInterval,
		GRPCMaxRecvMsgBytes:      parseEnvInt("GRPC_MAX_RECV_MSG_BYTES", 0),
		GRPCMaxSendMsgBytes:      parseEnvInt("GRPC_MAX_SEND_MSG_BYTES", 0),
		SchemaMaxFields:          parseEnvInt("SCHEMA_MAX_FIELDS", 10_000),
		SchemaMaxDocBytes:        parseEnvInt("SCHEMA_MAX_DOC_BYTES", 5*1024*1024),
		SchemaCompileTimeout:     compileTimeout,
		SchemaMaxRefDepth:        parseEnvInt("SCHEMA_MAX_REF_DEPTH", 64),
		InsecureListen:           getEnv("INSECURE_LISTEN", "") == "1",
		TLSCertFile:              getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:               getEnv("TLS_KEY_FILE", ""),
		TLSClientCAFile:          getEnv("TLS_CLIENT_CA_FILE", ""),
		TLSGatewayCAFile:         getEnv("TLS_GATEWAY_CA_FILE", ""),
		TLSGatewayServerName:     getEnv("TLS_GATEWAY_SERVER_NAME", ""),
		TLSGatewayClientCertFile: getEnv("TLS_GATEWAY_CLIENT_CERT_FILE", ""),
		TLSGatewayClientKeyFile:  getEnv("TLS_GATEWAY_CLIENT_KEY_FILE", ""),
		RateLimitEnabled:         getEnv("RATE_LIMIT_ENABLED", "true") != "false",
		RateLimitAnonRPS:         parseEnvFloat("RATE_LIMIT_ANON_RPS", 10),
		RateLimitAuthedRPS:       parseEnvFloat("RATE_LIMIT_AUTHED_RPS", 100),
		RateLimitSuperAdminRPS:   parseEnvFloat("RATE_LIMIT_SUPERADMIN_RPS", 0),
		RateLimitBurst:           parseEnvInt("RATE_LIMIT_BURST", 10),
	}
}

// parseEnvInt parses an integer env var. Returns fallback when unset or invalid;
// invalid values exit with an error so misconfiguration is loud, not silent.
func parseEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		slog.Error("invalid integer env var", "key", key, "value", raw, "error", err)
		os.Exit(1)
	}
	return v
}

func parseEnvFloat(key string, fallback float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		slog.Error("invalid float env var", "key", key, "value", raw, "error", err)
		os.Exit(1)
	}
	return v
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

func newLogger(level string) *slog.Logger {
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
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	return slog.New(telemetry.NewLogHandler(handler))
}
