package adminclient

import (
	"context"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
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
		resp, err := c.config.ListVersions(c.withAuth(ctx), &pb.ListVersionsRequest{
			TenantId:  tenantID,
			PageSize:  100,
			PageToken: pageToken,
		})
		if err != nil {
			return nil, mapError(err)
		}
		for _, v := range resp.Versions {
			all = append(all, versionFromProto(v))
		}
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
	resp, err := c.config.GetVersion(c.withAuth(ctx), &pb.GetVersionRequest{
		TenantId: tenantID,
		Version:  version,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return versionFromProto(resp.ConfigVersion), nil
}

// RollbackConfig creates a new config version with the values from a previous version.
// The description is optional — pass an empty string to use the default.
func (c *Client) RollbackConfig(ctx context.Context, tenantID string, targetVersion int32, description string) (*Version, error) {
	if c.config == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.config.RollbackToVersion(c.withAuth(ctx), &pb.RollbackToVersionRequest{
		TenantId:    tenantID,
		Version:     targetVersion,
		Description: ptrString(description),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return versionFromProto(resp.ConfigVersion), nil
}

// ExportConfig serializes a tenant's configuration to YAML.
// If version is nil, the latest version is exported.
func (c *Client) ExportConfig(ctx context.Context, tenantID string, version *int32) ([]byte, error) {
	if c.config == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.config.ExportConfig(c.withAuth(ctx), &pb.ExportConfigRequest{
		TenantId: tenantID,
		Version:  version,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return resp.YamlContent, nil
}

// ImportConfig applies configuration values from YAML, creating a new version.
// The description is optional — pass an empty string to use the default.
func (c *Client) ImportConfig(ctx context.Context, tenantID string, yamlContent []byte, description string) (*Version, error) {
	if c.config == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.config.ImportConfig(c.withAuth(ctx), &pb.ImportConfigRequest{
		TenantId:    tenantID,
		YamlContent: yamlContent,
		Description: ptrString(description),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return versionFromProto(resp.ConfigVersion), nil
}
