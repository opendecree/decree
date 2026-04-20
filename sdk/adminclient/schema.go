package adminclient

import (
	"context"
)

// CreateSchema creates a new schema with an initial draft version (v1).
// The name must be a valid slug (lowercase alphanumeric and hyphens, 1-63 chars).
// At least one field is required.
func (c *Client) CreateSchema(ctx context.Context, name string, fields []Field, description string) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.CreateSchema(ctx, &CreateSchemaRequest{
		Name:        name,
		Fields:      fields,
		Description: description,
	})
}

// GetSchema retrieves a schema by ID at its latest version.
func (c *Client) GetSchema(ctx context.Context, id string) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.GetSchema(ctx, id, nil)
}

// GetSchemaVersion retrieves a schema at a specific version.
func (c *Client) GetSchemaVersion(ctx context.Context, id string, version int32) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.GetSchema(ctx, id, &version)
}

// ListSchemas returns all schemas, auto-paginating through all results.
func (c *Client) ListSchemas(ctx context.Context) ([]*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	var all []*Schema
	pageToken := ""
	for {
		resp, err := c.schema.ListSchemas(ctx, 100, pageToken)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Schemas...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return all, nil
}

// UpdateSchema creates a new draft version by merging field changes with the latest version.
// Fields listed in addOrModify are added or updated. Fields listed in removeFields are removed.
func (c *Client) UpdateSchema(ctx context.Context, id string, addOrModify []Field, removeFields []string, versionDescription string) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.UpdateSchema(ctx, &UpdateSchemaRequest{
		ID:                 id,
		AddOrModify:        addOrModify,
		RemoveFields:       removeFields,
		VersionDescription: versionDescription,
	})
}

// PublishSchema marks a schema version as published and immutable.
// Only published versions can be assigned to tenants.
// Returns [ErrFailedPrecondition] if the version is already published.
func (c *Client) PublishSchema(ctx context.Context, id string, version int32) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.PublishSchema(ctx, id, version)
}

// DeleteSchema permanently deletes a schema and all its versions.
// This cascades to all tenants assigned to this schema.
func (c *Client) DeleteSchema(ctx context.Context, id string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	return c.schema.DeleteSchema(ctx, id)
}

// ExportSchema serializes a schema version to YAML.
// If version is nil, the latest version is exported.
func (c *Client) ExportSchema(ctx context.Context, id string, version *int32) ([]byte, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.ExportSchema(ctx, id, version)
}

// ImportSchema creates a schema (or new version) from YAML content.
// Full-replace semantics: the YAML defines the complete field set.
// Returns [ErrAlreadyExists] if the imported fields are identical to the latest version.
// Imported versions are always created as drafts (unpublished) unless autoPublish is true.
func (c *Client) ImportSchema(ctx context.Context, yamlContent []byte, autoPublish ...bool) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	ap := len(autoPublish) > 0 && autoPublish[0]
	return c.schema.ImportSchema(ctx, yamlContent, ap)
}

// GetLatestPublishedSchemaVersion finds the most recent published version of a schema by name.
// Returns the schema ID and version number. If the latest version is a draft, walks
// backward until it finds a published version.
// Returns [ErrNotFound] if no schema with the given name exists, or if no version is published.
func (c *Client) GetLatestPublishedSchemaVersion(ctx context.Context, name string) (id string, version int32, err error) {
	if c.schema == nil {
		return "", 0, ErrServiceNotConfigured
	}
	schemas, err := c.ListSchemas(ctx)
	if err != nil {
		return "", 0, err
	}
	var match *Schema
	for _, s := range schemas {
		if s.Name == name {
			match = s
			break
		}
	}
	if match == nil {
		return "", 0, ErrNotFound
	}
	if match.Published {
		return match.ID, match.Version, nil
	}
	// Latest is a draft — walk back to find the newest published version.
	for v := match.Version - 1; v >= 1; v-- {
		s, err := c.GetSchemaVersion(ctx, match.ID, v)
		if err != nil {
			return "", 0, err
		}
		if s.Published {
			return match.ID, v, nil
		}
	}
	return "", 0, ErrNotFound
}
