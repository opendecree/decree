// Package koanfcontrib provides a koanf provider backed by an OpenDecree
// configclient. It implements the koanf.Provider interface so that decree
// configuration values can be loaded directly into a koanf instance.
//
// Usage:
//
//	provider := koanfcontrib.New(client, "my-tenant")
//	k := koanf.New(".")
//	if err := k.Load(provider, nil); err != nil {
//	    log.Fatal(err)
//	}
package koanfcontrib

import (
	"context"
	"fmt"
	"time"

	"github.com/knadh/koanf/v2"
	"github.com/opendecree/decree/sdk/configclient"
)

// Provider is a koanf provider backed by a decree configclient.
// It fetches all configuration values for a tenant via [configclient.Client.GetAll]
// and exposes them as a flat map of field-path keys.
type Provider struct {
	client   *configclient.Client
	tenantID string
	timeout  time.Duration
}

// New creates a Provider for the given tenant.
// opts can be used to override defaults such as the per-call timeout.
func New(client *configclient.Client, tenantID string, opts ...Option) *Provider {
	p := &Provider{client: client, tenantID: tenantID, timeout: 5 * time.Second}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Option configures a Provider.
type Option func(*Provider)

// WithTimeout sets the per-call context timeout used by Read.
// The default timeout is 5 seconds.
func WithTimeout(d time.Duration) Option {
	return func(p *Provider) { p.timeout = d }
}

// Read fetches all configuration values for the tenant from OpenDecree and
// returns them as a flat map[string]interface{} keyed by field path.
// Implements koanf.Provider.
func (p *Provider) Read() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	all, err := p.client.GetAll(ctx, p.tenantID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{}, len(all))
	for k, v := range all {
		m[k] = v
	}
	return m, nil
}

// ReadBytes is not supported by this provider. Use Read instead (pass nil as
// the Parser argument to koanf.Load).
// Implements koanf.Provider.
func (p *Provider) ReadBytes() ([]byte, error) {
	return nil, fmt.Errorf("koanfcontrib: ReadBytes not supported; use Read with a nil parser")
}

// Watch polls for configuration changes at a 30-second interval by invoking
// the callback, which causes koanf to re-load the provider.
// The callback receives a nil event and nil error on each tick.
//
// Watch launches a background goroutine and returns immediately.
// This is a convenience method — it is not part of the koanf.Provider interface.
func (p *Provider) Watch(cb func(event interface{}, err error)) error {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			cb(nil, nil)
		}
	}()
	return nil
}

// Ensure *Provider implements koanf.Provider at compile time.
var _ koanf.Provider = (*Provider)(nil)
