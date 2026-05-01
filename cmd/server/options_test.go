package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"

	"github.com/opendecree/decree/internal/ratelimit"
	"github.com/opendecree/decree/internal/server"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func baseServerCfg() serverConfig {
	return serverConfig{
		EnableServices:      []string{"schema", "config", "audit"},
		GRPCMaxRecvMsgBytes: 4 << 20,
		GRPCMaxSendMsgBytes: 4 << 20,
	}
}

func newTestRateLimiter() *ratelimit.Interceptor {
	lim := ratelimit.NewInProcess(rate.Limit(1), 1)
	return ratelimit.New(ratelimit.Config{Authenticated: lim})
}

func TestBuildServerOptions_TLS(t *testing.T) {
	cfg := baseServerCfg()
	cfg.InsecureListen = false
	tlsCfg := &server.TLSConfig{CertFile: "cert.pem", KeyFile: "key.pem"}

	got := buildServerOptions(cfg, discardLogger(), nil, tlsCfg, nil)

	assert.True(t, got.UseTLS, "expected TLS branch")
	assert.False(t, got.UseInsecure, "expected insecure branch off")
	assert.False(t, got.HasRateLimiter, "rate limiter should not be wired when nil")
	assert.Len(t, got.Opts, 6, "5 base options + TLS option")
}

func TestBuildServerOptions_Insecure(t *testing.T) {
	cfg := baseServerCfg()
	cfg.InsecureListen = true

	got := buildServerOptions(cfg, discardLogger(), nil, nil, nil)

	assert.False(t, got.UseTLS)
	assert.True(t, got.UseInsecure)
	assert.Len(t, got.Opts, 6, "5 base options + Insecure option")
}

func TestBuildServerOptions_RateLimiterWired(t *testing.T) {
	cfg := baseServerCfg()
	cfg.InsecureListen = true

	got := buildServerOptions(cfg, discardLogger(), nil, nil, newTestRateLimiter())

	assert.True(t, got.HasRateLimiter, "non-nil rate limiter must be wired into the option slice")
	assert.Len(t, got.Opts, 7, "5 base options + Insecure + RateLimiter")
}

func TestBuildServerOptions_RateLimiterAbsent(t *testing.T) {
	cfg := baseServerCfg()
	cfg.InsecureListen = true

	got := buildServerOptions(cfg, discardLogger(), nil, nil, nil)

	assert.False(t, got.HasRateLimiter, "nil rate limiter must not produce a WithRateLimiter option")
	assert.Len(t, got.Opts, 6)
}

func TestBuildServerOptions_ExtraGRPCOpts(t *testing.T) {
	cfg := baseServerCfg()
	cfg.InsecureListen = true
	extra := []grpc.ServerOption{grpc.MaxConcurrentStreams(42)}

	got := buildServerOptions(cfg, discardLogger(), extra, nil, nil)

	assert.Len(t, got.Opts, 6, "extra grpc opts go inside WithGRPCServerOptions, not as separate options")
}

func TestBuildGatewayOptions_TLS(t *testing.T) {
	cfg := baseServerCfg()
	cfg.InsecureListen = false
	gwTLS := &server.GatewayTLSConfig{CAFile: "ca.pem"}

	got := buildGatewayOptions(cfg, discardLogger(), []byte(`{}`), gwTLS)

	assert.True(t, got.UseTLS)
	assert.False(t, got.UseInsecure)
	assert.Len(t, got.Opts, 5, "4 base options + TLS option")
}

func TestBuildGatewayOptions_Insecure(t *testing.T) {
	cfg := baseServerCfg()
	cfg.InsecureListen = true

	got := buildGatewayOptions(cfg, discardLogger(), []byte(`{}`), nil)

	assert.False(t, got.UseTLS)
	assert.True(t, got.UseInsecure)
	assert.Len(t, got.Opts, 5, "4 base options + Insecure option")
}
