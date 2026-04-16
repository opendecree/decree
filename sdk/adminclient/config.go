package adminclient

import (
	"context"
)

// ListConfigVersions returns all config versions for a tenant, newest first.
// Auto-paginates through all results.
func (c *Client) ListConfigVersions(ctx context.Context, tenantID string) ([]*Version, error) {
	if c.config == nil {
		return nil, ErrServiceNotConfigured
	}
	var all []*Version
	pageToken := ""
	for {
		resp, err := c.config.ListVersions(ctx, tenantID, 100, pageToken)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Versions...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return all, nil
}

// GetConfigVersion retrieves metadata for a specific config version.
func (c *Client) GetConfigVersion(ctx context.Context, tenantID string, version int32) (*Version, error) {
	if c.config == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.config.GetVersion(ctx, tenantID, version)
}

// RollbackConfig creates a new config version with the values from a previous version.
// The description is optional — pass an empty string to use the default.
func (c *Client) RollbackConfig(ctx context.Context, tenantID string, targetVersion int32, description string) (*Version, error) {
	if c.config == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.config.RollbackToVersion(ctx, tenantID, targetVersion, description)
}

// ExportConfig serializes a tenant's configuration to YAML.
// If version is nil, the latest version is exported.
func (c *Client) ExportConfig(ctx context.Context, tenantID string, version *int32) ([]byte, error) {
	if c.config == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.config.ExportConfig(ctx, tenantID, version)
}

// ImportMode controls how imported values interact with existing config.
type ImportMode int32

const (
	// ImportModeMerge updates fields from YAML that differ, keeps runtime overrides.
	ImportModeMerge ImportMode = 1
	// ImportModeReplace does a full replace — all fields from YAML, runtime overrides wiped.
	ImportModeReplace ImportMode = 2
	// ImportModeDefaults only sets fields that have no value yet.
	ImportModeDefaults ImportMode = 3
)

// ImportConfig applies configuration values from YAML, creating a new version.
// The description is optional — pass an empty string to use the default.
// Mode defaults to ImportModeMerge if not specified.
func (c *Client) ImportConfig(ctx context.Context, tenantID string, yamlContent []byte, description string, mode ...ImportMode) (*Version, error) {
	if c.config == nil {
		return nil, ErrServiceNotConfigured
	}
	m := ImportModeMerge
	if len(mode) > 0 {
		m = mode[0]
	}
	return c.config.ImportConfig(ctx, &ImportConfigRequest{
		TenantID:    tenantID,
		YamlContent: yamlContent,
		Description: description,
		Mode:        m,
	})
}
