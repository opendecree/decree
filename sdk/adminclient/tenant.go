package adminclient

import (
	"context"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// CreateTenant creates a new tenant assigned to a published schema version.
// The name must be a valid slug (lowercase alphanumeric and hyphens, 1-63 chars).
// Returns [ErrFailedPrecondition] if the schema version is not published.
func (c *Client) CreateTenant(ctx context.Context, name, schemaID string, schemaVersion int32) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.CreateTenant(c.withAuth(ctx), &pb.CreateTenantRequest{
		Name:          name,
		SchemaId:      schemaID,
		SchemaVersion: schemaVersion,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return tenantFromProto(resp.Tenant), nil
}

// GetTenant retrieves a tenant by ID.
func (c *Client) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.GetTenant(c.withAuth(ctx), &pb.GetTenantRequest{Id: id})
	if err != nil {
		return nil, mapError(err)
	}
	return tenantFromProto(resp.Tenant), nil
}

// ListTenants returns all tenants, optionally filtered by schema ID.
// Pass an empty schemaID to list all tenants. Auto-paginates through all results.
func (c *Client) ListTenants(ctx context.Context, schemaID string) ([]*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	var all []*Tenant
	pageToken := ""
	for {
		req := &pb.ListTenantsRequest{
			PageSize:  100,
			PageToken: pageToken,
		}
		if schemaID != "" {
			req.SchemaId = &schemaID
		}
		resp, err := c.schema.ListTenants(c.withAuth(ctx), req)
		if err != nil {
			return nil, mapError(err)
		}
		for _, t := range resp.Tenants {
			all = append(all, tenantFromProto(t))
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return all, nil
}

// UpdateTenantName updates a tenant's name.
// The new name must be a valid slug.
func (c *Client) UpdateTenantName(ctx context.Context, id, newName string) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.UpdateTenant(c.withAuth(ctx), &pb.UpdateTenantRequest{
		Id:   id,
		Name: &newName,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return tenantFromProto(resp.Tenant), nil
}

// UpdateTenantSchema upgrades a tenant to a new schema version.
// The new version must belong to the same schema and must be published.
func (c *Client) UpdateTenantSchema(ctx context.Context, id string, schemaVersion int32) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.UpdateTenant(c.withAuth(ctx), &pb.UpdateTenantRequest{
		Id:            id,
		SchemaVersion: &schemaVersion,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return tenantFromProto(resp.Tenant), nil
}

// DeleteTenant permanently deletes a tenant and all its configuration data.
func (c *Client) DeleteTenant(ctx context.Context, id string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	_, err := c.schema.DeleteTenant(c.withAuth(ctx), &pb.DeleteTenantRequest{Id: id})
	return mapError(err)
}
