package adminclient

import (
	"context"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// CreateSchema creates a new schema with an initial draft version (v1).
// The name must be a valid slug (lowercase alphanumeric and hyphens, 1-63 chars).
// At least one field is required.
func (c *Client) CreateSchema(ctx context.Context, name string, fields []Field, description string) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.CreateSchema(c.withAuth(ctx), &pb.CreateSchemaRequest{
		Name:        name,
		Description: ptrString(description),
		Fields:      fieldsToProto(fields),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return schemaFromProto(resp.Schema), nil
}

// GetSchema retrieves a schema by ID at its latest version.
func (c *Client) GetSchema(ctx context.Context, id string) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.GetSchema(c.withAuth(ctx), &pb.GetSchemaRequest{Id: id})
	if err != nil {
		return nil, mapError(err)
	}
	return schemaFromProto(resp.Schema), nil
}

// GetSchemaVersion retrieves a schema at a specific version.
func (c *Client) GetSchemaVersion(ctx context.Context, id string, version int32) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.GetSchema(c.withAuth(ctx), &pb.GetSchemaRequest{
		Id:      id,
		Version: &version,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return schemaFromProto(resp.Schema), nil
}

// ListSchemas returns all schemas, auto-paginating through all results.
func (c *Client) ListSchemas(ctx context.Context) ([]*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	var all []*Schema
	pageToken := ""
	for {
		resp, err := c.schema.ListSchemas(c.withAuth(ctx), &pb.ListSchemasRequest{
			PageSize:  100,
			PageToken: pageToken,
		})
		if err != nil {
			return nil, mapError(err)
		}
		for _, s := range resp.Schemas {
			all = append(all, schemaFromProto(s))
		}
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
	resp, err := c.schema.UpdateSchema(c.withAuth(ctx), &pb.UpdateSchemaRequest{
		Id:                 id,
		VersionDescription: ptrString(versionDescription),
		Fields:             fieldsToProto(addOrModify),
		RemoveFields:       removeFields,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return schemaFromProto(resp.Schema), nil
}

// PublishSchema marks a schema version as published and immutable.
// Only published versions can be assigned to tenants.
// Returns [ErrFailedPrecondition] if the version is already published.
func (c *Client) PublishSchema(ctx context.Context, id string, version int32) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.PublishSchema(c.withAuth(ctx), &pb.PublishSchemaRequest{
		Id:      id,
		Version: version,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return schemaFromProto(resp.Schema), nil
}

// DeleteSchema permanently deletes a schema and all its versions.
// This cascades to all tenants assigned to this schema.
func (c *Client) DeleteSchema(ctx context.Context, id string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	_, err := c.schema.DeleteSchema(c.withAuth(ctx), &pb.DeleteSchemaRequest{Id: id})
	return mapError(err)
}

// ExportSchema serializes a schema version to YAML.
// If version is nil, the latest version is exported.
func (c *Client) ExportSchema(ctx context.Context, id string, version *int32) ([]byte, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.ExportSchema(c.withAuth(ctx), &pb.ExportSchemaRequest{
		Id:      id,
		Version: version,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return resp.YamlContent, nil
}

// ImportSchema creates a schema (or new version) from YAML content.
// Full-replace semantics: the YAML defines the complete field set.
// Returns [ErrAlreadyExists] if the imported fields are identical to the latest version.
// Imported versions are always created as drafts (unpublished).
func (c *Client) ImportSchema(ctx context.Context, yamlContent []byte) (*Schema, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.ImportSchema(c.withAuth(ctx), &pb.ImportSchemaRequest{
		YamlContent: yamlContent,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return schemaFromProto(resp.Schema), nil
}
