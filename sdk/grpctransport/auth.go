package grpctransport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
// Ignored when [WithBearerToken] or [WithTokenSource] is set.
func WithSubject(s string) Option {
	return func(c *config) { c.auth.subject = s }
}

// WithRole sets the x-role metadata header.
// Ignored when [WithBearerToken] or [WithTokenSource] is set.
func WithRole(r string) Option {
	return func(c *config) { c.auth.role = r }
}

// WithTenantID sets the x-tenant-id metadata header.
// Ignored when [WithBearerToken] or [WithTokenSource] is set.
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

// bearerToken implements [credentials.PerRPCCredentials] for a static JWT.
// RequireTransportSecurity returns true so gRPC refuses to send the token
// over a plaintext connection.
type bearerToken struct{ token string }

func (b bearerToken) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + b.token}, nil
}

func (b bearerToken) RequireTransportSecurity() bool { return true }

// tokenSourceCreds implements [credentials.PerRPCCredentials] for a dynamic
// token source. The source is called on every RPC so short-lived tokens
// (OAuth2, expiring JWTs) are always fresh.
// RequireTransportSecurity returns true so gRPC refuses to send the token
// over a plaintext connection.
type tokenSourceCreds struct {
	source func(context.Context) (string, error)
}

func (t tokenSourceCreds) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	tok, err := t.source(ctx)
	if err != nil {
		return nil, err
	}
	if tok == "" {
		// Skip setting the Authorization header rather than sending a malformed
		// "Bearer " credential. The server will treat the request as unauthenticated.
		return nil, nil
	}
	return map[string]string{"authorization": "Bearer " + tok}, nil
}

func (t tokenSourceCreds) RequireTransportSecurity() bool { return true }

// applyAuth injects authentication metadata into the outgoing gRPC context
// and returns any per-RPC call options required.
//
// For bearer-token and token-source auth, it returns a
// [grpc.PerRPCCredentials] call option backed by a
// [credentials.PerRPCCredentials] implementation whose
// RequireTransportSecurity method returns true — gRPC will refuse to send
// the credential if the connection is not TLS-protected.
//
// For metadata-header auth (x-subject / x-role / x-tenant-id), headers are
// appended to the existing outgoing metadata so the caller's metadata is
// preserved.
func applyAuth(ctx context.Context, auth authConfig) (context.Context, []grpc.CallOption, error) {
	switch {
	case auth.tokenSource != nil:
		creds := tokenSourceCreds{source: auth.tokenSource}
		return ctx, []grpc.CallOption{grpc.PerRPCCredentials(creds)}, nil
	case auth.bearerToken != "":
		creds := bearerToken{token: auth.bearerToken}
		return ctx, []grpc.CallOption{grpc.PerRPCCredentials(creds)}, nil
	default:
		pairs := make([]string, 0, 6)
		if auth.subject != "" {
			pairs = append(pairs, "x-subject", auth.subject)
		}
		if auth.role != "" {
			pairs = append(pairs, "x-role", auth.role)
		}
		if auth.tenantID != "" {
			pairs = append(pairs, "x-tenant-id", auth.tenantID)
		}
		if len(pairs) > 0 {
			ctx = metadata.AppendToOutgoingContext(ctx, pairs...)
		}
		return ctx, nil, nil
	}
}

// authApplier holds pre-computed auth state so that per-RPC credential
// objects and metadata key-value pairs are built once at construction time
// rather than on every method call.
type authApplier struct {
	mdPairs  []string          // pre-computed metadata key-value pairs (metadata-header auth)
	callOpts []grpc.CallOption // pre-computed PerRPCCredentials option (bearer/token-source auth)
}

// newAuthApplier converts an authConfig into a pre-computed authApplier.
func newAuthApplier(auth authConfig) authApplier {
	switch {
	case auth.tokenSource != nil:
		creds := tokenSourceCreds{source: auth.tokenSource}
		return authApplier{callOpts: []grpc.CallOption{grpc.PerRPCCredentials(creds)}}
	case auth.bearerToken != "":
		creds := bearerToken{token: auth.bearerToken}
		return authApplier{callOpts: []grpc.CallOption{grpc.PerRPCCredentials(creds)}}
	default:
		var pairs []string
		if auth.subject != "" {
			pairs = append(pairs, "x-subject", auth.subject)
		}
		if auth.role != "" {
			pairs = append(pairs, "x-role", auth.role)
		}
		if auth.tenantID != "" {
			pairs = append(pairs, "x-tenant-id", auth.tenantID)
		}
		return authApplier{mdPairs: pairs}
	}
}

// apply attaches the pre-computed auth to the outgoing context and returns
// any required call options. It never returns an error.
func (a authApplier) apply(ctx context.Context) (context.Context, []grpc.CallOption) {
	if len(a.mdPairs) > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, a.mdPairs...)
	}
	return ctx, a.callOpts
}

// Compile-time check: both types satisfy credentials.PerRPCCredentials.
var (
	_ credentials.PerRPCCredentials = bearerToken{}
	_ credentials.PerRPCCredentials = tokenSourceCreds{}
)
