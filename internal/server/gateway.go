package server

import (
	"context"
	"errors"
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
// It is optional — only started when httpPort is non-empty.
type Gateway struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewGateway creates a new HTTP gateway that proxies to the given gRPC address.
// Returns nil if httpPort is empty (gateway disabled). Either WithGatewayTLS
// or WithGatewayInsecure must be supplied.
func NewGateway(ctx context.Context, httpPort, grpcAddr string, opts ...GatewayOption) (*Gateway, error) {
	if httpPort == "" {
		return nil, nil
	}

	o := gatewayOptions{logger: slog.Default()}
	for _, opt := range opts {
		opt(&o)
	}

	if o.tls == nil && !o.insecure {
		return nil, errors.New("gateway TLS config is required; pass WithGatewayTLS or WithGatewayInsecure for local dev (INSECURE_LISTEN=1)")
	}
	if o.tls != nil && o.insecure {
		return nil, errors.New("WithGatewayTLS and WithGatewayInsecure are mutually exclusive")
	}

	recvCap := o.maxRecvMsgBytes
	if recvCap <= 0 {
		recvCap = DefaultMaxMsgBytes
	}
	sendCap := o.maxSendMsgBytes
	if sendCap <= 0 {
		sendCap = DefaultMaxMsgBytes
	}

	var transportCreds grpc.DialOption
	if o.insecure {
		transportCreds = grpc.WithTransportCredentials(insecure.NewCredentials())
	} else {
		creds, err := o.tls.ClientCredentials()
		if err != nil {
			return nil, fmt.Errorf("build gateway TLS credentials: %w", err)
		}
		transportCreds = grpc.WithTransportCredentials(creds)
	}

	conn, err := grpc.NewClient(
		grpcAddr,
		transportCreds,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(recvCap),
			grpc.MaxCallSendMsgSize(sendCap),
		),
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
		pb.RegisterServerServiceHandler,
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
	if len(o.openAPISpec) > 0 {
		top := http.NewServeMux()
		top.HandleFunc("GET /docs/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(o.openAPISpec)
		})
		top.HandleFunc("GET /docs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(swaggerUIPage))
		})
		top.Handle("/", mux)
		handler = top
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", httpPort),
		Handler: handler,
	}

	return &Gateway{
		httpServer: httpServer,
		logger:     o.logger,
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
