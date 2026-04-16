package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

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

// Config holds the server configuration.
type Config struct {
	GRPCPort        string
	EnableServices  []string
	Logger          *slog.Logger
	AuthInterceptor GRPCInterceptor
	ExtraOptions    []grpc.ServerOption
}

// Server wraps the gRPC server and health service.
type Server struct {
	grpcServer   *grpc.Server
	healthServer *health.Server
	listener     net.Listener
	config       Config
}

// New creates a new gRPC server with interceptors.
func New(cfg Config) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		return nil, fmt.Errorf("listen on port %s: %w", cfg.GRPCPort, err)
	}

	opts := make([]grpc.ServerOption, 0, len(cfg.ExtraOptions)+2)
	opts = append(opts, cfg.ExtraOptions...)
	opts = append(opts,
		grpc.ChainUnaryInterceptor(cfg.AuthInterceptor.UnaryInterceptor()),
		grpc.ChainStreamInterceptor(cfg.AuthInterceptor.StreamInterceptor()),
	)

	grpcServer := grpc.NewServer(opts...)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	reflection.Register(grpcServer)

	return &Server{
		grpcServer:   grpcServer,
		healthServer: healthServer,
		listener:     listener,
		config:       cfg,
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
	s.config.Logger.InfoContext(ctx, "gRPC server listening", "port", s.config.GRPCPort)
	return s.grpcServer.Serve(s.listener)
}

// GracefulStop gracefully stops the gRPC server.
func (s *Server) GracefulStop(ctx context.Context) {
	s.config.Logger.InfoContext(ctx, "shutting down gRPC server")
	s.healthServer.Shutdown()
	s.grpcServer.GracefulStop()
}

// IsServiceEnabled checks if a service name is in the enabled list.
func (s *Server) IsServiceEnabled(name string) bool {
	for _, svc := range s.config.EnableServices {
		if svc == name {
			return true
		}
	}
	return false
}
