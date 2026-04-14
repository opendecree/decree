package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// Gateway is an HTTP reverse proxy that translates REST/JSON requests to gRPC.
// It is optional — only started when HTTPPort is configured.
type Gateway struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// GatewayConfig holds configuration for the HTTP gateway.
type GatewayConfig struct {
	// HTTPPort is the port the gateway listens on. Empty means gateway is disabled.
	HTTPPort string
	// GRPCAddr is the gRPC server address to proxy to (e.g. "localhost:9090").
	GRPCAddr string
	// Logger for gateway operations.
	Logger *slog.Logger
	// OpenAPISpec is the raw OpenAPI JSON spec to serve at /docs/openapi.json.
	// If nil, the docs endpoints are not registered.
	OpenAPISpec []byte
}

// NewGateway creates a new HTTP gateway that proxies to the given gRPC address.
// Returns nil if HTTPPort is empty (gateway disabled).
func NewGateway(ctx context.Context, cfg GatewayConfig) (*Gateway, error) {
	if cfg.HTTPPort == "" {
		return nil, nil
	}

	conn, err := grpc.NewClient(
		cfg.GRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial gRPC server: %w", err)
	}

	mux := runtime.NewServeMux(
		runtime.WithMetadata(forwardAuthHeaders),
		runtime.WithHealthzEndpoint(nil),
	)

	// Register all services.
	handlers := []func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error{
		pb.RegisterVersionServiceHandler,
		pb.RegisterSchemaServiceHandler,
		pb.RegisterConfigServiceHandler,
		pb.RegisterAuditServiceHandler,
	}
	for _, register := range handlers {
		if err := register(ctx, mux, conn); err != nil {
			return nil, fmt.Errorf("register gateway handler: %w", err)
		}
	}

	// Wrap gateway mux with docs routes.
	handler := http.Handler(mux)
	if len(cfg.OpenAPISpec) > 0 {
		top := http.NewServeMux()
		top.HandleFunc("GET /docs/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(cfg.OpenAPISpec)
		})
		top.HandleFunc("GET /docs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(swaggerUIPage))
		})
		top.Handle("/", mux)
		handler = top
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler: handler,
	}

	return &Gateway{
		httpServer: httpServer,
		logger:     cfg.Logger,
	}, nil
}

// Serve starts the HTTP gateway. Blocks until stopped.
func (g *Gateway) Serve(ctx context.Context) error {
	g.logger.InfoContext(ctx, "HTTP gateway listening", "port", strings.TrimPrefix(g.httpServer.Addr, ":"))
	err := g.httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully stops the HTTP gateway.
func (g *Gateway) Shutdown(ctx context.Context) {
	g.logger.InfoContext(ctx, "shutting down HTTP gateway")
	_ = g.httpServer.Shutdown(ctx)
}

// forwardAuthHeaders extracts auth-related HTTP headers and forwards them as
// gRPC metadata. This enables the same auth interceptors to work for both
// gRPC and REST clients.
func forwardAuthHeaders(ctx context.Context, req *http.Request) metadata.MD {
	md := metadata.MD{}
	for _, header := range []string{"x-subject", "x-role", "x-tenant-id", "authorization"} {
		if v := req.Header.Get(header); v != "" {
			md.Set(header, v)
		}
	}
	return md
}

// swaggerUIPage is a self-contained HTML page that renders the OpenAPI spec
// using Swagger UI loaded from unpkg CDN.
const swaggerUIPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>OpenDecree API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>html{box-sizing:border-box;overflow-y:scroll}*,*:before,*:after{box-sizing:inherit}body{margin:0;background:#fafafa}</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/docs/openapi.json",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "StandaloneLayout"
    });
  </script>
</body>
</html>`
