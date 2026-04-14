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
	"google.golang.org/grpc/metadata"
)

func TestNewGateway_DisabledWhenNoPort(t *testing.T) {
	gw, err := NewGateway(context.Background(), GatewayConfig{
		HTTPPort: "",
		GRPCAddr: "localhost:9090",
		Logger:   slog.Default(),
	})
	require.NoError(t, err)
	assert.Nil(t, gw, "gateway should be nil when HTTPPort is empty")
}

func TestNewGateway_CreatesWithValidConfig(t *testing.T) {
	gw, err := NewGateway(context.Background(), GatewayConfig{
		HTTPPort: "0",
		GRPCAddr: "localhost:9090",
		Logger:   slog.Default(),
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestGateway_ServeAndShutdown(t *testing.T) {
	gw, err := NewGateway(context.Background(), GatewayConfig{
		HTTPPort: "0",
		GRPCAddr: "localhost:9090",
		Logger:   slog.Default(),
	})
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- gw.Serve(context.Background()) }()

	// Give Serve time to bind.
	time.Sleep(50 * time.Millisecond)

	// Shutdown should return cleanly.
	gw.Shutdown(context.Background())
	assert.NoError(t, <-errCh)
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
	gw, err := NewGateway(context.Background(), GatewayConfig{
		HTTPPort:    "0",
		GRPCAddr:    "localhost:9090",
		Logger:      slog.Default(),
		OpenAPISpec: spec,
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestNewGateway_WithoutOpenAPISpec(t *testing.T) {
	gw, err := NewGateway(context.Background(), GatewayConfig{
		HTTPPort: "0",
		GRPCAddr: "localhost:9090",
		Logger:   slog.Default(),
	})
	require.NoError(t, err)
	assert.NotNil(t, gw)
}

func TestGateway_DocsEndpoints(t *testing.T) {
	spec := []byte(`{"swagger":"2.0","info":{"title":"test"}}`)
	gw, err := NewGateway(context.Background(), GatewayConfig{
		HTTPPort:    "0",
		GRPCAddr:    "localhost:9090",
		Logger:      slog.Default(),
		OpenAPISpec: spec,
	})
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

	t.Run("openapi spec", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/openapi.json", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), `"swagger"`)
	})
}

func TestVersionService_GetServerVersion(t *testing.T) {
	svc := &VersionService{}
	resp, err := svc.GetServerVersion(context.Background(), nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	// version.Version and version.Commit are set at build time; empty in tests is fine.
	assert.IsType(t, "", resp.Version)
	assert.IsType(t, "", resp.Commit)
}

// Verify that metadata.MD satisfies the grpc metadata interface.
var _ metadata.MD = forwardAuthHeaders(context.Background(), &http.Request{Header: http.Header{}})
