package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc/metadata"
)

func TestNewGateway_DisabledWhenNoPort(t *testing.T) {
	gw, err := NewGateway(context.Background(), "", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
	)
	require.NoError(t, err)
	assert.Nil(t, gw, "gateway should be nil when httpPort is empty")
}

func TestNewGateway_CreatesWithValidConfig(t *testing.T) {
	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
	)
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestNewGateway_RequiresTLSOrInsecure(t *testing.T) {
	_, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway TLS config is required")
}

func TestGateway_ServeAndShutdown(t *testing.T) {
	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
	)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- gw.Serve(context.Background()) }()

	// Give Serve time to bind.
	time.Sleep(50 * time.Millisecond)

	// Shutdown should return cleanly.
	gw.Shutdown(context.Background())
	assert.NoError(t, <-errCh)
}

func TestGateway_ShutdownClosesGRPCConn(t *testing.T) {
	// Snapshot goroutines before creating the gateway so that any leaked by
	// other tests in the suite do not cause a false failure here.
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
	)
	require.NoError(t, err)

	gw.Shutdown(context.Background())
}

func TestForwardAuthHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected map[string]string
	}{
		{
			name:     "all auth headers",
			headers:  map[string]string{"x-subject": "admin", "x-role": "superadmin", "x-tenant-id": "t1", "authorization": "Bearer tok"},
			expected: map[string]string{"x-subject": "admin", "x-role": "superadmin", "x-tenant-id": "t1", "authorization": "Bearer tok"},
		},
		{
			name:     "partial headers",
			headers:  map[string]string{"x-subject": "user1"},
			expected: map[string]string{"x-subject": "user1"},
		},
		{
			name:     "no auth headers",
			headers:  map[string]string{"content-type": "application/json"},
			expected: map[string]string{},
		},
		{
			name:     "empty",
			headers:  map[string]string{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://localhost/v1/version", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			md := forwardAuthHeaders(context.Background(), req)
			for k, v := range tt.expected {
				vals := md.Get(k)
				require.Len(t, vals, 1, "expected metadata key %q", k)
				assert.Equal(t, v, vals[0])
			}

			// Verify no extra keys forwarded.
			expectedKeys := make(map[string]bool)
			for k := range tt.expected {
				expectedKeys[k] = true
			}
			for k := range md {
				assert.True(t, expectedKeys[k], "unexpected metadata key %q", k)
			}
		})
	}
}

func TestForwardAuthHeaders_CaseInsensitive(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/v1/version", nil)
	req.Header.Set("X-Subject", "admin")
	req.Header.Set("X-Role", "superadmin")

	md := forwardAuthHeaders(context.Background(), req)
	assert.Equal(t, []string{"admin"}, md.Get("x-subject"))
	assert.Equal(t, []string{"superadmin"}, md.Get("x-role"))
}

func TestNewGateway_WithOpenAPISpec(t *testing.T) {
	spec := []byte(`{"openapi":"3.0.0","info":{"title":"test"}}`)
	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithOpenAPISpec(spec),
		WithGatewayInsecure(),
	)
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestNewGateway_WithoutOpenAPISpec(t *testing.T) {
	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
	)
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestGateway_DocsEndpoints(t *testing.T) {
	spec := []byte(`{"swagger":"2.0","info":{"title":"test"}}`)
	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithOpenAPISpec(spec),
		WithGatewayInsecure(),
	)
	require.NoError(t, err)

	// Use the gateway's handler directly via httptest to avoid port binding.
	handler := gw.httpServer.Handler

	t.Run("swagger UI", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		assert.Contains(t, w.Body.String(), "swagger-ui")
	})

	t.Run("swagger UI has CSP header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		csp := w.Header().Get("Content-Security-Policy")
		assert.NotEmpty(t, csp, "CSP header must be set on /docs")
		assert.Contains(t, csp, "script-src 'self'")
	})

	t.Run("swagger UI references local assets not CDN", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		body := w.Body.String()
		assert.NotContains(t, body, "unpkg.com", "must not load from CDN")
		assert.Contains(t, body, "/docs/swaggerui/", "must reference vendored assets")
	})

	t.Run("vendored swagger-ui bundle served", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/swaggerui/swagger-ui-bundle.js", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		assert.NotEmpty(t, w.Body.Bytes())
	})

	t.Run("vendored swagger-ui css served", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/swaggerui/swagger-ui.css", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("openapi spec", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/openapi.json", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), `"swagger"`)
	})
}

func TestGateway_DocsProtected(t *testing.T) {
	spec := []byte(`{"swagger":"2.0","info":{"title":"test"}}`)
	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithOpenAPISpec(spec),
		WithGatewayInsecure(),
		WithGatewayDocsProtected(),
	)
	require.NoError(t, err)
	handler := gw.httpServer.Handler

	t.Run("docs blocked without auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("spec blocked without auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/openapi.json", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("docs allowed with auth header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs", nil)
		req.Header.Set("Authorization", "Bearer tok")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestCORSMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	t.Run("allowed origin gets CORS headers", func(t *testing.T) {
		h := corsMiddleware([]string{"https://example.com"})(ok)
		req := httptest.NewRequest("GET", "/v1/config", nil)
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("unlisted origin gets no CORS headers", func(t *testing.T) {
		h := corsMiddleware([]string{"https://example.com"})(ok)
		req := httptest.NewRequest("GET", "/v1/config", nil)
		req.Header.Set("Origin", "https://evil.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("preflight OPTIONS returns 204", func(t *testing.T) {
		h := corsMiddleware([]string{"https://example.com"})(ok)
		req := httptest.NewRequest("OPTIONS", "/v1/config", nil)
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("wildcard never set", func(t *testing.T) {
		h := corsMiddleware([]string{"https://a.com", "https://b.com"})(ok)
		req := httptest.NewRequest("GET", "/v1/config", nil)
		req.Header.Set("Origin", "https://c.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.NotEqual(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("no origin header passes through unchanged", func(t *testing.T) {
		h := corsMiddleware([]string{"https://example.com"})(ok)
		req := httptest.NewRequest("GET", "/v1/config", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestServerService_GetServerInfo(t *testing.T) {
	svc := NewServerService(Features{
		Schema:        true,
		Config:        true,
		Audit:         false,
		UsageTracking: true,
		JWTAuth:       false,
		HTTPGateway:   true,
	})
	resp, err := svc.GetServerInfo(context.Background(), nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.IsType(t, "", resp.Version)
	assert.IsType(t, "", resp.Commit)
	assert.True(t, resp.Features["schema"])
	assert.True(t, resp.Features["config"])
	assert.False(t, resp.Features["audit"])
	assert.True(t, resp.Features["usage_tracking"])
	assert.False(t, resp.Features["jwt_auth"])
	assert.True(t, resp.Features["http_gateway"])
	assert.Len(t, resp.Features, 6)
}

// Verify that metadata.MD satisfies the grpc metadata interface.
var _ metadata.MD = forwardAuthHeaders(context.Background(), &http.Request{Header: http.Header{}})

func TestRejectAuthHeadersMiddleware_BlocksAuthHeaders(t *testing.T) {
	blocked := []string{"x-subject", "x-role", "x-tenant-id"}
	for _, h := range blocked {
		t.Run(h, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/config", nil)
			req.Header.Set(h, "somevalue")
			w := httptest.NewRecorder()
			rejectAuthHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "DECREE_GATEWAY_TRUSTED_PROXY")
		})
	}
}

func TestRejectAuthHeadersMiddleware_AllowsOtherHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/config", nil)
	req.Header.Set("authorization", "Bearer token")
	req.Header.Set("content-type", "application/json")
	w := httptest.NewRecorder()
	rejectAuthHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRejectAuthHeadersMiddleware_CaseInsensitive(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/config", nil)
	req.Header.Set("X-Subject", "admin") // capital letters
	w := httptest.NewRecorder()
	rejectAuthHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestNewGateway_HTTPServerTimeoutsSet(t *testing.T) {
	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
	)
	require.NoError(t, err)
	require.NotNil(t, gw)

	s := gw.httpServer
	assert.Equal(t, 10*time.Second, s.ReadHeaderTimeout)
	assert.Equal(t, 30*time.Second, s.ReadTimeout)
	assert.Equal(t, 60*time.Second, s.WriteTimeout)
	assert.Equal(t, 120*time.Second, s.IdleTimeout)
	assert.Equal(t, 1<<20, s.MaxHeaderBytes)
}

func TestNewGateway_RefusesPlaintextNonLoopback(t *testing.T) {
	// Port "8080" binds to 0.0.0.0 — all-interfaces, not loopback.
	_, err := NewGateway(context.Background(), "8080", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to serve plaintext HTTP")
}

func TestNewGateway_PlaintextTerminatorAllowsNonLoopback(t *testing.T) {
	gw, err := NewGateway(context.Background(), "8080", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
		WithGatewayPlaintextTerminator(),
	)
	require.NoError(t, err)
	assert.NotNil(t, gw)
	gw.Shutdown(context.Background())
}

func TestNewGateway_ServerTLSEnablesHTTPS(t *testing.T) {
	dir := t.TempDir()
	bundle := genCertBundle(t, "localhost", false)
	_, certFile, keyFile := bundle.writeFiles(t, dir, "gw")

	gw, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
		WithGatewayServerTLS(&TLSConfig{CertFile: certFile, KeyFile: keyFile}),
	)
	require.NoError(t, err)
	require.NotNil(t, gw)
	assert.True(t, gw.serveTLS)
	assert.NotNil(t, gw.httpServer.TLSConfig)
	assert.NotNil(t, gw.httpServer.TLSConfig.GetCertificate)
}

func TestNewGateway_ServerTLSRejectsBadCert(t *testing.T) {
	_, err := NewGateway(context.Background(), "0", "localhost:9090",
		WithGatewayLogger(slog.Default()),
		WithGatewayInsecure(),
		WithGatewayServerTLS(&TLSConfig{CertFile: "/nonexistent/cert.pem", KeyFile: "/nonexistent/key.pem"}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build gateway server TLS")
}

func TestIsLoopbackAddr(t *testing.T) {
	tests := []struct {
		addr     string
		loopback bool
	}{
		{":0", true},     // ephemeral — test port
		{":8080", false}, // all-interfaces
		{"127.0.0.1:8080", true},
		{"::1:8080", false},       // invalid host:port but won't panic
		{"localhost:8080", false}, // DNS name, not an IP
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			assert.Equal(t, tt.loopback, isLoopbackAddr(tt.addr))
		})
	}
}

func TestIsStreamingPath(t *testing.T) {
	tests := []struct {
		path       string
		wantStream bool
	}{
		{"/v1/tenants/abc/config:subscribe", true},
		{"/v1/tenants/00000000-0000-0000-0000-000000000001/config:subscribe", true},
		{"/v1/tenants/abc/config", false},
		{"/v1/tenants/abc/config:get", false},
		{"/v1/schemas", false},
		{"/docs", false},
		{"", false},
		// Must not match a path that merely contains ":subscribe" mid-segment.
		{"/v1/config:subscribe/extra", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.wantStream, isStreamingPath(tt.path))
		})
	}
}

// clearWriteDeadlineMiddleware_NonStreamingPassthrough verifies that non-streaming
// routes pass through without modification.
func TestClearWriteDeadlineMiddleware_NonStreamingPassthrough(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := clearWriteDeadlineMiddleware(inner)
	req := httptest.NewRequest("GET", "/v1/tenants/abc/config", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestClearWriteDeadlineMiddleware_StreamingPath verifies that streaming routes
// clear the write deadline (SetWriteDeadline is called on the ResponseController).
// httptest.ResponseRecorder does not implement SetWriteDeadline, so the error is
// silently ignored by the middleware — the key check is that the inner handler
// is still called and the response completes.
func TestClearWriteDeadlineMiddleware_StreamingPath(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := clearWriteDeadlineMiddleware(inner)
	req := httptest.NewRequest("GET", "/v1/tenants/abc/config:subscribe", nil)
	w := httptest.NewRecorder()
	// The middleware calls rc.SetWriteDeadline on the recorder; ResponseRecorder
	// does not implement net.Conn so the call returns an unsupported error that is
	// intentionally ignored. The important invariant is that the inner handler runs.
	h.ServeHTTP(w, req)
	assert.True(t, called, "inner handler must still be called for streaming paths")
	assert.Equal(t, http.StatusOK, w.Code)
}
