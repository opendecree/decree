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
	return retry(ctx, c, func(ctx context.Context) (*Tenant, error) {
		return c.schema.GetTenant(ctx, id)
	})
}

// ListTenantsPage returns a single page of tenants, optionally filtered by schema ID.
// Pass an empty schemaID to list all tenants. pageSize must be positive.
// pageToken is the cursor from a previous call; pass "" for the first page.
// Returns the page contents and the next page token (empty string when no more pages remain).
// Memory usage is bounded to one page at a time.
func (c *Client) ListTenantsPage(ctx context.Context, schemaID string, pageSize int32, pageToken string) ([]*Tenant, string, error) {
	if c.schema == nil {
		return nil, "", ErrServiceNotConfigured
	}
	var schemaFilter *string
	if schemaID != "" {
		schemaFilter = &schemaID
	}
	resp, err := retry(ctx, c, func(ctx context.Context) (*ListTenantsResponse, error) {
		return c.schema.ListTenants(ctx, schemaFilter, pageSize, pageToken)
	})
	if err != nil {
		return nil, "", err
	}
	return resp.Tenants, resp.NextPageToken, nil
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
		resp, err := retry(ctx, c, func(ctx context.Context) (*ListTenantsResponse, error) {
			return c.schema.ListTenants(ctx, schemaFilter, 100, pageToken)
		})
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

// UpdateTenantOption configures an UpdateTenant call.
type UpdateTenantOption func(*updateTenantOptions)

type updateTenantOptions struct {
	name          *string
	schemaVersion *int32
}

// WithTenantName sets the new name for a tenant.
func WithTenantName(name string) UpdateTenantOption {
	return func(o *updateTenantOptions) { o.name = &name }
}

// WithTenantSchemaVersion sets the schema version to upgrade the tenant to.
func WithTenantSchemaVersion(version int32) UpdateTenantOption {
	return func(o *updateTenantOptions) { o.schemaVersion = &version }
}

// UpdateTenant updates a tenant's name and/or schema version in a single call.
// At least one option must be provided. Use [WithTenantName] and/or
// [WithTenantSchemaVersion] to specify what to change.
func (c *Client) UpdateTenant(ctx context.Context, id string, opts ...UpdateTenantOption) (*Tenant, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	var o updateTenantOptions
	for _, opt := range opts {
		opt(&o)
	}
	return c.schema.UpdateTenant(ctx, &UpdateTenantRequest{
		ID:            id,
		Name:          o.name,
		SchemaVersion: o.schemaVersion,
	})
}

// DeleteTenant permanently deletes a tenant and all its configuration data.
func (c *Client) DeleteTenant(ctx context.Context, id string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	return c.schema.DeleteTenant(ctx, id)
}
