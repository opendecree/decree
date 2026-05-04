package main

import (
	"log/slog"

	"google.golang.org/grpc"

	"github.com/opendecree/decree/internal/ratelimit"
	"github.com/opendecree/decree/internal/server"
)

// serverOptionsBuild captures the option slice handed to server.New plus the
// boolean decisions that drove it. Tests assert on the decisions so a flag
// silently dropped (read but never wired into an option) is caught.
type serverOptionsBuild struct {
	Opts           []server.Option
	UseTLS         bool
	UseInsecure    bool
	HasRateLimiter bool
	HasReflection  bool
}

func buildServerOptions(
	cfg serverConfig,
	logger *slog.Logger,
	extraGRPCOpts []grpc.ServerOption,
	serverTLS *server.TLSConfig,
	rl *ratelimit.Interceptor,
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
	Opts        []server.GatewayOption
	UseTLS      bool
	UseInsecure bool
}

func buildGatewayOptions(
	cfg serverConfig,
	logger *slog.Logger,
	openAPISpec []byte,
	gwTLS *server.GatewayTLSConfig,
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
		out.Opts = append(out.Opts, server.WithGatewayInsecure())
		out.UseInsecure = true
	} else {
		out.Opts = append(out.Opts, server.WithGatewayTLS(gwTLS))
		out.UseTLS = true
	}
	return out
}
