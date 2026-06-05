// Package vipercontrib provides a Viper remote config provider backed by a
// decree configclient. It allows Go applications using Viper for configuration
// to transparently read values from OpenDecree.
//
// Usage:
//
//	transport := grpctransport.NewConfigTransport(conn)
//	client := configclient.New(transport)
//	p := vipercontrib.New(client, "my-tenant")
//	vipercontrib.Register("decree", p)
//
//	// Then use Viper normally:
//	viper.SetConfigType("json")
//	viper.AddRemoteProvider("decree", "", "")
//	viper.ReadRemoteConfig()
package vipercontrib

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
	"github.com/spf13/viper"
)

// Provider is a Viper remote config provider backed by a decree configclient.
// It fetches all config fields for the tenant and maps them to Viper keys.
type Provider struct {
	client   *configclient.Client
	tenantID string
	timeout  time.Duration
}

// New creates a new Provider for the given tenant.
func New(client *configclient.Client, tenantID string, opts ...Option) *Provider {
	p := &Provider{
		client:   client,
		tenantID: tenantID,
		timeout:  5 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Option configures a Provider.
type Option func(*Provider)

// WithTimeout sets the request timeout for config reads.
func WithTimeout(d time.Duration) Option {
	return func(p *Provider) { p.timeout = d }
}

// Register registers this provider with Viper under the given provider name and
// appends that name to viper.SupportedRemoteProviders so that
// viper.AddRemoteProvider accepts it.
//
// Call Register before viper.AddRemoteProvider.
func Register(name string, p *Provider) {
	viper.SupportedRemoteProviders = append(viper.SupportedRemoteProviders, name)
	viper.RemoteConfig = &configProvider{p: p}
}

type configProvider struct {
	p *Provider
}

func (cp *configProvider) Get(_ viper.RemoteProvider) (io.Reader, error) {
	return cp.p.fetch()
}

func (cp *configProvider) Watch(_ viper.RemoteProvider) (io.Reader, error) {
	return cp.p.fetch()
}

func (cp *configProvider) WatchChannel(_ viper.RemoteProvider) (<-chan *viper.RemoteResponse, chan bool) {
	ch := make(chan *viper.RemoteResponse)
	quit := make(chan bool)
	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				r, err := cp.p.fetch()
				if err == nil {
					b, _ := io.ReadAll(r)
					ch <- &viper.RemoteResponse{Value: b}
				}
				time.Sleep(30 * time.Second)
			}
		}
	}()
	return ch, quit
}

// fetch retrieves all config values for the tenant and encodes them as JSON.
// Viper parses JSON from remote providers. Field paths with dot separators
// (e.g. "app.name") are converted to nested JSON objects so that Viper's
// hierarchical key lookup works correctly:
//
//	"app.name" → {"app": {"name": "..."}}
func (p *Provider) fetch() (io.Reader, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	m, err := p.client.GetAll(ctx, p.tenantID)
	if err != nil {
		return nil, err
	}

	nested := toNestedMap(m)

	b, err := json.Marshal(nested)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

// toNestedMap converts a flat map with dot-separated keys into a nested
// map[string]any suitable for Viper's hierarchical key lookup.
//
// Example:
//
//	{"app.name": "myapp", "app.debug": "true"} →
//	{"app": {"name": "myapp", "debug": "true"}}
func toNestedMap(flat map[string]string) map[string]any {
	out := make(map[string]any)
	for key, val := range flat {
		setNested(out, key, val)
	}
	return out
}

// setNested inserts val into m at the path described by the dot-separated key.
func setNested(m map[string]any, key, val string) {
	for {
		dot := -1
		for i := 0; i < len(key); i++ {
			if key[i] == '.' {
				dot = i
				break
			}
		}
		if dot < 0 {
			m[key] = val
			return
		}
		prefix := key[:dot]
		key = key[dot+1:]
		child, ok := m[prefix]
		if !ok {
			next := make(map[string]any)
			m[prefix] = next
			m = next
			continue
		}
		next, ok := child.(map[string]any)
		if !ok {
			// Conflict: existing leaf at prefix — prefer the deeper key.
			next = make(map[string]any)
			m[prefix] = next
		}
		m = next
	}
}
