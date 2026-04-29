package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// GRPCInterceptor provides unary and stream interceptors for gRPC.
type GRPCInterceptor interface {
	UnaryInterceptor() grpc.UnaryServerInterceptor
	StreamInterceptor() grpc.StreamServerInterceptor
}

// DefaultMaxMsgBytes is applied to inbound and outbound gRPC messages when
// WithMaxRecvMsgBytes / WithMaxSendMsgBytes are zero or negative.
const DefaultMaxMsgBytes = 20 * 1024 * 1024

// Server wraps the gRPC server and health service.
type Server struct {
	grpcServer     *grpc.Server
	healthServer   *health.Server
	listener       net.Listener
	logger         *slog.Logger
	grpcPort       string
	enableServices []string
}

// New creates a new gRPC server with interceptors. grpcPort and auth are
// required; pass options for everything else. Either WithTLS or WithInsecure
// must be supplied.
func New(grpcPort string, auth GRPCInterceptor, opts ...Option) (*Server, error) {
	o := options{logger: slog.Default()}
	for _, opt := range opts {
		opt(&o)
	}

	if o.tls == nil && !o.insecure {
		return nil, errors.New("TLS config is required; pass WithTLS or WithInsecure for local dev (INSECURE_LISTEN=1)")
	}
	if o.tls != nil && o.insecure {
		return nil, errors.New("WithTLS and WithInsecure are mutually exclusive")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		return nil, fmt.Errorf("listen on port %s: %w", grpcPort, err)
	}

	recvCap := o.maxRecvMsgBytes
	if recvCap <= 0 {
		recvCap = DefaultMaxMsgBytes
	}
	sendCap := o.maxSendMsgBytes
	if sendCap <= 0 {
		sendCap = DefaultMaxMsgBytes
	}

	grpcOpts := make([]grpc.ServerOption, 0, len(o.extraOptions)+5)
	grpcOpts = append(grpcOpts, o.extraOptions...)
	if o.tls != nil {
		creds, err := o.tls.ServerCredentials()
		if err != nil {
			_ = listener.Close()
			return nil, fmt.Errorf("build TLS credentials: %w", err)
		}
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
	}
	// Recovery wraps all middleware. Auth runs second. Rate limiter (when set) runs after auth.
	unaryChain := []grpc.UnaryServerInterceptor{
		recoveryUnaryInterceptor(o.logger),
		auth.UnaryInterceptor(),
	}
	streamChain := []grpc.StreamServerInterceptor{
		recoveryStreamInterceptor(o.logger),
		auth.StreamInterceptor(),
	}
	if o.rateLimiter != nil {
		unaryChain = append(unaryChain, o.rateLimiter.UnaryInterceptor())
		streamChain = append(streamChain, o.rateLimiter.StreamInterceptor())
	}
	grpcOpts = append(grpcOpts,
		grpc.MaxRecvMsgSize(recvCap),
		grpc.MaxSendMsgSize(sendCap),
		grpc.ChainUnaryInterceptor(unaryChain...),
		grpc.ChainStreamInterceptor(streamChain...),
	)

	grpcServer := grpc.NewServer(grpcOpts...)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	reflection.Register(grpcServer)

	return &Server{
		grpcServer:     grpcServer,
		healthServer:   healthServer,
		listener:       listener,
		logger:         o.logger,
		grpcPort:       grpcPort,
		enableServices: o.enableServices,
	}, nil
}

// GRPCServer returns the underlying grpc.Server for service registration.
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// SetServiceHealthy marks a service as healthy.
func (s *Server) SetServiceHealthy(service string) {
	s.healthServer.SetServingStatus(service, healthpb.HealthCheckResponse_SERVING)
}

// Serve starts the gRPC server. Blocks until stopped.
func (s *Server) Serve(ctx context.Context) error {
	s.logger.InfoContext(ctx, "gRPC server listening", "port", s.grpcPort)
	return s.grpcServer.Serve(s.listener)
}

// GracefulStop gracefully stops the gRPC server.
func (s *Server) GracefulStop(ctx context.Context) {
	s.logger.InfoContext(ctx, "shutting down gRPC server")
	s.healthServer.Shutdown()
	s.grpcServer.GracefulStop()
}

// IsServiceEnabled checks if a service name is in the enabled list.
func (s *Server) IsServiceEnabled(name string) bool {
	return slices.Contains(s.enableServices, name)
}
