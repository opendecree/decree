package main

import (
	"io/fs"
	"log/slog"

	"google.golang.org/grpc"

	"github.com/opendecree/decree/internal/ratelimit"
	"github.com/opendecree/decree/internal/server"
)

// serverOptionsBuild captures the option slice handed to server.New plus the
// boolean decisions that drove it. Tests assert on the decisions so a flag
// silently dropped (read but never wired into an option) is caught.
type serverOptionsBuild struct {
	Opts              []server.Option
	UseTLS            bool
	UseInsecure       bool
	HasPreAuthLimiter bool
	HasRateLimiter    bool
	HasReflection     bool
}

func buildServerOptions(
	cfg serverConfig,
	logger *slog.Logger,
	extraGRPCOpts []grpc.ServerOption,
	serverTLS *server.TLSConfig,
	rl *ratelimit.Interceptor,
	preAuth *ratelimit.Interceptor,
) serverOptionsBuild {
	out := serverOptionsBuild{
		Opts: []server.Option{
			server.WithEnableServices(cfg.EnableServices),
			server.WithLogger(logger),
			server.WithGRPCServerOptions(extraGRPCOpts...),
			server.WithMaxRecvMsgBytes(cfg.GRPCMaxRecvMsgBytes),
			server.WithMaxSendMsgBytes(cfg.GRPCMaxSendMsgBytes),
		},
	}
	if cfg.InsecureListen {
		out.Opts = append(out.Opts, server.WithInsecure())
		out.UseInsecure = true
	} else {
		out.Opts = append(out.Opts, server.WithTLS(serverTLS))
		out.UseTLS = true
	}
	if preAuth != nil {
		out.Opts = append(out.Opts, server.WithPreAuthLimiter(preAuth))
		out.HasPreAuthLimiter = true
	}
	if rl != nil {
		out.Opts = append(out.Opts, server.WithRateLimiter(rl))
		out.HasRateLimiter = true
	}
	if cfg.EnableReflection {
		out.Opts = append(out.Opts, server.WithReflection())
		out.HasReflection = true
	}
	return out
}

// gatewayOptionsBuild mirrors serverOptionsBuild for the HTTP gateway.
type gatewayOptionsBuild struct {
	Opts                   []server.GatewayOption
	UseTLS                 bool
	UseInsecure            bool
	HasUI                  bool
	HasTrustedProxy        bool
	HasPlaintextTerminator bool
}

func buildGatewayOptions(
	cfg serverConfig,
	logger *slog.Logger,
	openAPISpec []byte,
	gwTLS *server.GatewayTLSConfig,
	uiFS fs.FS,
) gatewayOptionsBuild {
	out := gatewayOptionsBuild{
		Opts: []server.GatewayOption{
			server.WithGatewayLogger(logger),
			server.WithOpenAPISpec(openAPISpec),
			server.WithGatewayMaxRecvMsgBytes(cfg.GRPCMaxRecvMsgBytes),
			server.WithGatewayMaxSendMsgBytes(cfg.GRPCMaxSendMsgBytes),
		},
	}
	if cfg.InsecureListen {
		out.Opts = append(out.Opts,
			server.WithGatewayInsecure(),
			// INSECURE_LISTEN=1 is already an explicit opt-in for running without
			// TLS, so acknowledge the plaintext HTTP listener on all interfaces.
			server.WithGatewayPlaintextTerminator(),
		)
		out.UseInsecure = true
		out.HasPlaintextTerminator = true
	} else {
		out.Opts = append(out.Opts, server.WithGatewayTLS(gwTLS))
		out.UseTLS = true
	}
	if uiFS != nil {
		out.Opts = append(out.Opts, server.WithUI(uiFS))
		out.HasUI = true
	}
	if cfg.GatewayTrustedProxy {
		out.Opts = append(out.Opts, server.WithGatewayTrustedProxy())
		out.HasTrustedProxy = true
	}
	return out
}
