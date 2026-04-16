package configclient

import (
	"context"
)

// Snapshot provides read access to configuration values pinned to a specific version.
// This guarantees all reads within a flow see a consistent point-in-time view,
// even if the configuration is being updated concurrently.
type Snapshot struct {
	client   *Client
	tenantID string
	version  int32
}

// Snapshot creates a read-only view pinned to the tenant's current latest version.
// All subsequent reads on the returned Snapshot use this version, ensuring consistency
// across multiple Get calls within the same flow.
func (c *Client) Snapshot(ctx context.Context, tenantID string) (*Snapshot, error) {
	resp, err := c.transport.GetConfig(ctx, &GetConfigRequest{
		TenantID: tenantID,
	})
	if err != nil {
		return nil, err
	}
	return &Snapshot{client: c, tenantID: tenantID, version: resp.Version}, nil
}

// AtVersion creates a read-only view pinned to the specified config version.
// No network call is made — the version is used on subsequent reads.
func (c *Client) AtVersion(tenantID string, version int32) *Snapshot {
	return &Snapshot{client: c, tenantID: tenantID, version: version}
}

// Version returns the pinned version number.
func (s *Snapshot) Version() int32 {
	return s.version
}

// Get returns the value of a single field at the pinned version.
// Returns [ErrNotFound] if the field has no value at this version.
func (s *Snapshot) Get(ctx context.Context, fieldPath string) (string, error) {
	resp, err := s.client.transport.GetField(ctx, &GetFieldRequest{
		TenantID:  s.tenantID,
		FieldPath: fieldPath,
		Version:   &s.version,
	})
	if err != nil {
		return "", err
	}
	return resp.Value.String(), nil
}

// GetAll returns all configuration values at the pinned version.
func (s *Snapshot) GetAll(ctx context.Context) (map[string]string, error) {
	v := s.version
	resp, err := s.client.transport.GetConfig(ctx, &GetConfigRequest{
		TenantID: s.tenantID,
		Version:  &v,
	})
	if err != nil {
		return nil, err
	}
	return configToMap(resp), nil
}

// GetFields returns the values for the specified field paths at the pinned version.
// Fields that have no value at this version are omitted from the result.
func (s *Snapshot) GetFields(ctx context.Context, fieldPaths []string) (map[string]string, error) {
	resp, err := s.client.transport.GetFields(ctx, &GetFieldsRequest{
		TenantID:   s.tenantID,
		FieldPaths: fieldPaths,
		Version:    &s.version,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(resp.Values))
	for _, v := range resp.Values {
		result[v.FieldPath] = v.Value.String()
	}
	return result, nil
}
