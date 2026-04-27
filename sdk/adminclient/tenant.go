package adminclient

import (
	"context"
)

// CreateTenant creates a new tenant assigned to a published schema version.
// The name must be a valid slug (lowercase alphanumeric and hyphens, 1-63 chars).
// Returns [ErrFailedPrecondition] if the schema version is not published.
func (c *Client) CreateTenant(ctx context.Context, name, schemaID string, schemaVersion int32) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.CreateTenant(ctx, &CreateTenantRequest{
		Name:          name,
		SchemaID:      schemaID,
		SchemaVersion: schemaVersion,
	})
}

// GetTenant retrieves a tenant by ID.
func (c *Client) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.GetTenant(ctx, id)
}

// ListTenants returns all tenants, optionally filtered by schema ID.
// Pass an empty schemaID to list all tenants. Auto-paginates through all results.
func (c *Client) ListTenants(ctx context.Context, schemaID string) ([]*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	var schemaFilter *string
	if schemaID != "" {
		schemaFilter = &schemaID
	}
	var all []*Tenant
	pageToken := ""
	for {
		resp, err := c.schema.ListTenants(ctx, schemaFilter, 100, pageToken)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Tenants...)
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
	return c.schema.UpdateTenant(ctx, &UpdateTenantRequest{
		ID:   id,
		Name: &newName,
	})
}

// UpdateTenantSchema upgrades a tenant to a new schema version.
// The new version must belong to the same schema and must be published.
func (c *Client) UpdateTenantSchema(ctx context.Context, id string, schemaVersion int32) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.UpdateTenant(ctx, &UpdateTenantRequest{
		ID:            id,
		SchemaVersion: &schemaVersion,
	})
}

// UpdateTenant updates a tenant's name and/or schema version in a single call.
// Pass nil for fields that should be left unchanged. At least one field must be non-nil.
func (c *Client) UpdateTenant(ctx context.Context, id string, name *string, schemaVersion *int32) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.UpdateTenant(ctx, &UpdateTenantRequest{
		ID:            id,
		Name:          name,
		SchemaVersion: schemaVersion,
	})
}

// DeleteTenant permanently deletes a tenant and all its configuration data.
func (c *Client) DeleteTenant(ctx context.Context, id string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	return c.schema.DeleteTenant(ctx, id)
}
