package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/server/swaggerui"
)

// Gateway is an HTTP reverse proxy that translates REST/JSON requests to gRPC.
// It is optional — only started when httpPort is non-empty.
type Gateway struct {
	httpServer *http.Server
	conn       *grpc.ClientConn
	logger     *slog.Logger
	serveTLS   bool
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

	// Refuse to expose a plaintext HTTP listener on a non-loopback address unless
	// the caller explicitly acknowledges a TLS-terminating proxy is in front or
	// TLS is configured on the gateway listener itself.
	listenerAddr := fmt.Sprintf(":%s", httpPort)
	if o.serverTLS == nil && !o.plaintextTerminator && !isLoopbackAddr(listenerAddr) {
		return nil, fmt.Errorf(
			"refusing to serve plaintext HTTP on %s (non-loopback); "+
				"use WithGatewayServerTLS for HTTPS, or WithGatewayPlaintextTerminator "+
				"to acknowledge a TLS-terminating proxy is in front",
			listenerAddr,
		)
	}

	// Build gateway-listener TLS config when WithGatewayServerTLS is set.
	var serverTLSCfg *tls.Config
	if o.serverTLS != nil {
		if _, err := tls.LoadX509KeyPair(o.serverTLS.CertFile, o.serverTLS.KeyFile); err != nil {
			return nil, fmt.Errorf("build gateway server TLS: %w", err)
		}
		minVer := tlsMinVersion()
		serverTLSCfg = &tls.Config{
			GetCertificate: certLoader(o.serverTLS.CertFile, o.serverTLS.KeyFile),
			MinVersion:     minVer,
		}
		if minVer == tls.VersionTLS12 {
			serverTLSCfg.CipherSuites = tls12CipherSuites
		}
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

	// Wrap gateway mux with docs and UI routes.
	handler := http.Handler(mux)
	if len(o.openAPISpec) > 0 || o.uiFS != nil {
		top := http.NewServeMux()
		if len(o.openAPISpec) > 0 {
			// Serve vendored Swagger UI static assets from the embedded FS.
			assetsFS, _ := fs.Sub(swaggerui.Assets, "assets")
			top.Handle("GET /docs/swaggerui/", http.StripPrefix("/docs/swaggerui/", http.FileServerFS(assetsFS)))

			specHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(o.openAPISpec)
			})
			docsHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Content-Security-Policy",
					"default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
				_, _ = w.Write([]byte(swaggerUIPage))
			})

			if o.docsProtected {
				top.Handle("GET /docs/openapi.json", requireAuth(specHandler))
				top.Handle("GET /docs", requireAuth(docsHandler))
			} else {
				top.HandleFunc("GET /docs/openapi.json", specHandler.ServeHTTP)
				top.HandleFunc("GET /docs", docsHandler.ServeHTTP)
			}
		}
		if o.uiFS != nil {
			top.Handle("/admin/", http.StripPrefix("/admin", spaHandler(o.uiFS)))
		}
		top.Handle("/", mux)
		handler = top
	}

	if !o.trustedProxy {
		handler = rejectAuthHeadersMiddleware(handler)
	}

	if len(o.corsOrigins) > 0 {
		handler = corsMiddleware(o.corsOrigins)(handler)
	}

	httpServer := &http.Server{
		Addr:              listenerAddr,
		Handler:           handler,
		TLSConfig:         serverTLSCfg,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	return &Gateway{
		httpServer: httpServer,
		conn:       conn,
		logger:     o.logger,
		serveTLS:   serverTLSCfg != nil,
	}, nil
}

// Serve starts the HTTP gateway. Blocks until stopped.
func (g *Gateway) Serve(ctx context.Context) error {
	g.logger.InfoContext(ctx, "HTTP gateway listening", "port", strings.TrimPrefix(g.httpServer.Addr, ":"))
	var err error
	if g.serveTLS {
		// Empty cert/key: GetCertificate on TLSConfig handles every handshake.
		err = g.httpServer.ListenAndServeTLS("", "")
	} else {
		err = g.httpServer.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the HTTP gateway.
func (g *Gateway) Shutdown(ctx context.Context) {
	g.logger.InfoContext(ctx, "shutting down HTTP gateway")
	_ = g.httpServer.Shutdown(ctx)
	_ = g.conn.Close()
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

// rejectAuthHeadersMiddleware blocks requests that carry x-subject, x-role, or
// x-tenant-id headers. In the default (no trusted proxy) mode these headers are
// the sole source of identity, so allowing clients to set them directly enables
// impersonation. Wrap the handler with this middleware unless
// WithGatewayTrustedProxy is set.
func rejectAuthHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range []string{"x-subject", "x-role", "x-tenant-id"} {
			if r.Header.Get(h) != "" {
				http.Error(w,
					"auth headers (x-subject, x-role, x-tenant-id) are not accepted from clients; "+
						"set DECREE_GATEWAY_TRUSTED_PROXY=1 if a trusted proxy injects these headers",
					http.StatusUnauthorized,
				)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware sets Access-Control-Allow-Origin only for origins in the
// allowlist. Wildcard * is never used so that credentials can be included
// safely. Non-listed origins receive no CORS headers (browser blocks them).
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Subject, X-Role, X-Tenant-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireAuth rejects requests without an Authorization header with 401.
func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spaHandler serves an embedded filesystem for a single-page application.
// Requests for files that exist are served directly; all other paths fall back
// to index.html so client-side routing works correctly.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fsys.Open(path); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// isLoopbackAddr reports whether addr (host:port) resolves to a loopback
// interface. An empty host (all-interfaces) is not loopback. Port "0"
// (ephemeral, used in tests) is treated as loopback-safe.
func isLoopbackAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if port == "0" {
		return true
	}
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// swaggerUIPage is a self-contained HTML page that renders the OpenAPI spec
// using vendored Swagger UI assets (no CDN dependency).
const swaggerUIPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>OpenDecree API</title>
  <link rel="stylesheet" href="/docs/swaggerui/swagger-ui.css">
  <style>html{box-sizing:border-box;overflow-y:scroll}*,*:before,*:after{box-sizing:inherit}body{margin:0;background:#fafafa}</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="/docs/swaggerui/swagger-ui-bundle.js"></script>
  <script src="/docs/swaggerui/swagger-ui-standalone-preset.js"></script>
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
