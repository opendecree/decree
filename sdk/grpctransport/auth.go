// Package grpctransport implements the configclient.Transport,
// adminclient.SchemaTransport, adminclient.ConfigTransport, and
// adminclient.AuditTransport interfaces using gRPC.
package grpctransport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/configwatcher"
)

// Option configures gRPC transport behavior and pass-through options.
type Option func(*config)

type config struct {
	auth        authConfig
	clientOpts  []configclient.Option
	watcherOpts []configwatcher.Option
}

type authConfig struct {
	subject     string
	role        string
	tenantID    string
	bearerToken string
	tokenSource func(context.Context) (string, error)
}

// WithSubject sets the x-subject metadata header.
func WithSubject(s string) Option {
	return func(c *config) { c.auth.subject = s }
}

// WithRole sets the x-role metadata header.
func WithRole(r string) Option {
	return func(c *config) { c.auth.role = r }
}

// WithTenantID sets the x-tenant-id metadata header.
func WithTenantID(id string) Option {
	return func(c *config) { c.auth.tenantID = id }
}

// WithBearerToken sets a JWT bearer token for the authorization header.
// When set, the x-subject/x-role/x-tenant-id headers are not sent.
func WithBearerToken(token string) Option {
	return func(c *config) { c.auth.bearerToken = token }
}

// WithTokenSource sets a per-RPC token source. The function is called on
// every RPC and its return value is used as the Bearer token. Use this
// instead of [WithBearerToken] when tokens can expire (e.g. OAuth2,
// short-lived JWTs).
func WithTokenSource(fn func(context.Context) (string, error)) Option {
	return func(c *config) { c.auth.tokenSource = fn }
}

// WithRetry passes a retry configuration through to configclient.
func WithRetry(cfg configclient.RetryConfig) Option {
	return func(c *config) {
		c.clientOpts = append(c.clientOpts, configclient.WithRetry(cfg))
	}
}

// WithReconnectBackoff passes reconnect backoff configuration through to configwatcher.
func WithReconnectBackoff(min, max time.Duration) Option {
	return func(c *config) {
		c.watcherOpts = append(c.watcherOpts, configwatcher.WithReconnectBackoff(min, max))
	}
}

// WithLogger passes a logger through to configwatcher.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) {
		c.watcherOpts = append(c.watcherOpts, configwatcher.WithLogger(l))
	}
}

// ErrRoleRequired is returned by transport constructors when neither
// WithRole, WithBearerToken, nor WithTokenSource is provided.
var ErrRoleRequired = errors.New("grpctransport: WithRole is required when not using WithBearerToken or WithTokenSource")

func buildConfig(opts []Option) (config, error) {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.auth.bearerToken == "" && cfg.auth.tokenSource == nil && cfg.auth.role == "" {
		return config{}, ErrRoleRequired
	}
	return cfg, nil
}

// applyAuth injects authentication metadata into the outgoing gRPC context.
func applyAuth(ctx context.Context, auth authConfig) (context.Context, error) {
	md := metadata.MD{}
	switch {
	case auth.tokenSource != nil:
		tok, err := auth.tokenSource(ctx)
		if err != nil {
			return ctx, err
		}
		md.Set("authorization", "Bearer "+tok)
	case auth.bearerToken != "":
		md.Set("authorization", "Bearer "+auth.bearerToken)
	default:
		if auth.subject != "" {
			md.Set("x-subject", auth.subject)
		}
		if auth.role != "" {
			md.Set("x-role", auth.role)
		}
		if auth.tenantID != "" {
			md.Set("x-tenant-id", auth.tenantID)
		}
	}
	return metadata.NewOutgoingContext(ctx, md), nil
}
