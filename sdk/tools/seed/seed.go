// Package seed bootstraps an OpenDecree environment from a single YAML file
// containing a schema definition, tenant, configuration values, and optional
// field locks. The operation is idempotent: existing schemas with identical
// fields are skipped, existing tenants are reused, and config is merged.
//
// The [File] type also serves as the shared YAML format for the dump package.
package seed

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/opendecree/decree/sdk/adminclient"
)

// --- Seed/dump YAML format ---

// File is the top-level YAML document for seed and dump operations.
type File struct {
	SpecVersion string    `yaml:"spec_version"`
	Schema      SchemaDef `yaml:"schema"`
	Tenant      TenantDef `yaml:"tenant"`
	Config      ConfigDef `yaml:"config,omitempty"`
	Locks       []LockDef `yaml:"locks,omitempty"`
}

// SchemaDef defines a schema within a seed file.
type SchemaDef struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description,omitempty"`
	Info        *SchemaInfoDef      `yaml:"info,omitempty"`
	Fields      map[string]FieldDef `yaml:"fields"`
}

// SchemaInfoDef contains optional schema-level metadata.
type SchemaInfoDef struct {
	Title   string            `yaml:"title,omitempty"`
	Author  string            `yaml:"author,omitempty"`
	Contact *SchemaContactDef `yaml:"contact,omitempty"`
	Labels  map[string]string `yaml:"labels,omitempty"`
}

// SchemaContactDef contains contact information for a schema owner.
type SchemaContactDef struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
	URL   string `yaml:"url,omitempty"`
}

// FieldDef describes a single schema field.
type FieldDef struct {
	Type         string                `yaml:"type"`
	Description  string                `yaml:"description,omitempty"`
	Default      string                `yaml:"default,omitempty"`
	Nullable     bool                  `yaml:"nullable,omitempty"`
	Deprecated   bool                  `yaml:"deprecated,omitempty"`
	RedirectTo   string                `yaml:"redirect_to,omitempty"`
	Constraints  *ConstraintsDef       `yaml:"constraints,omitempty"`
	Title        string                `yaml:"title,omitempty"`
	Example      string                `yaml:"example,omitempty"`
	Examples     map[string]ExampleDef `yaml:"examples,omitempty"`
	ExternalDocs *ExternalDocsDef      `yaml:"externalDocs,omitempty"`
	Tags         []string              `yaml:"tags,omitempty"`
	Format       string                `yaml:"format,omitempty"`
	ReadOnly     bool                  `yaml:"readOnly,omitempty"`
	WriteOnce    bool                  `yaml:"writeOnce,omitempty"`
	Sensitive    bool                  `yaml:"sensitive,omitempty"`
}

// ExampleDef represents a named example value.
type ExampleDef struct {
	Value   string `yaml:"value"`
	Summary string `yaml:"summary,omitempty"`
}

// ExternalDocsDef links to external documentation.
type ExternalDocsDef struct {
	Description string `yaml:"description,omitempty"`
	URL         string `yaml:"url"`
}

// ConstraintsDef uses OAS-style naming for field constraints.
type ConstraintsDef struct {
	Minimum          *float64 `yaml:"minimum,omitempty"`
	Maximum          *float64 `yaml:"maximum,omitempty"`
	ExclusiveMinimum *float64 `yaml:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `yaml:"exclusiveMaximum,omitempty"`
	MinLength        *int32   `yaml:"minLength,omitempty"`
	MaxLength        *int32   `yaml:"maxLength,omitempty"`
	Pattern          string   `yaml:"pattern,omitempty"`
	Enum             []string `yaml:"enum,omitempty"`
	JSONSchema       string   `yaml:"json_schema,omitempty"`
}

// TenantDef defines the tenant to create.
type TenantDef struct {
	Name string `yaml:"name"`
}

// ConfigDef defines initial configuration values.
type ConfigDef struct {
	Description string                    `yaml:"description,omitempty"`
	Values      map[string]ConfigValueDef `yaml:"values,omitempty"`
}

// ConfigValueDef represents a single config value.
type ConfigValueDef struct {
	Value       any    `yaml:"value"`
	Description string `yaml:"description,omitempty"`
}

// LockDef defines a field lock.
type LockDef struct {
	FieldPath    string   `yaml:"field_path"`
	LockedValues []string `yaml:"locked_values,omitempty"`
}

// --- Client interface ---

// Client defines the adminclient methods used by seed operations.
// The [adminclient.Client] type satisfies this interface.
type Client interface {
	ImportSchema(ctx context.Context, yamlContent []byte, autoPublish ...bool) (*adminclient.Schema, error)
	ListSchemas(ctx context.Context) ([]*adminclient.Schema, error)
	ListTenants(ctx context.Context, schemaID string) ([]*adminclient.Tenant, error)
	CreateTenant(ctx context.Context, name, schemaID string, schemaVersion int32) (*adminclient.Tenant, error)
	ImportConfig(ctx context.Context, tenantID string, yamlContent []byte, description string, mode ...adminclient.ImportMode) (*adminclient.Version, error)
	LockField(ctx context.Context, tenantID, fieldPath string, lockedValues ...string) error
}

// --- Parse ---

// ParseFile parses and validates a seed YAML file.
func ParseFile(data []byte) (*File, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if f.SpecVersion != "v1" {
		return nil, fmt.Errorf("unsupported spec_version: %q (expected \"v1\")", f.SpecVersion)
	}
	if f.Schema.Name == "" {
		return nil, fmt.Errorf("schema.name is required")
	}
	if len(f.Schema.Fields) == 0 {
		return nil, fmt.Errorf("schema.fields must have at least one field")
	}
	if f.Tenant.Name == "" {
		return nil, fmt.Errorf("tenant.name is required")
	}
	return &f, nil
}

// Marshal serializes a seed/dump file to YAML.
func Marshal(f *File) ([]byte, error) {
	return yaml.Marshal(f)
}

// --- Seed execution ---

// Result reports what happened during seeding.
type Result struct {
	SchemaID       string
	SchemaVersion  int32
	SchemaCreated  bool // false if skipped (already existed with same fields)
	TenantID       string
	TenantCreated  bool // false if reused existing tenant
	ConfigVersion  int32
	ConfigImported bool
	LocksApplied   int
}

// Option configures seed behavior.
type Option func(*options)

type options struct {
	autoPublish bool
}

// AutoPublish publishes the schema version after creation.
func AutoPublish() Option {
	return func(o *options) { o.autoPublish = true }
}

// Run executes the seed operation against a live server.
// It is idempotent: existing schemas with identical fields are skipped,
// existing tenants are reused, and config uses merge mode.
func Run(ctx context.Context, client Client, file *File, opts ...Option) (*Result, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	result := &Result{}

	// 1. Import schema.
	schemaYAML, err := marshalSchemaYAML(file)
	if err != nil {
		return nil, fmt.Errorf("marshaling schema: %w", err)
	}

	schema, err := client.ImportSchema(ctx, schemaYAML, o.autoPublish)
	if err != nil {
		if errors.Is(err, adminclient.ErrAlreadyExists) {
			// Schema exists with identical fields — find it.
			schemas, listErr := client.ListSchemas(ctx)
			if listErr != nil {
				return nil, fmt.Errorf("listing schemas: %w", listErr)
			}
			for _, s := range schemas {
				if s.Name == file.Schema.Name {
					result.SchemaID = s.ID
					result.SchemaVersion = s.Version
					break
				}
			}
			if result.SchemaID == "" {
				return nil, fmt.Errorf("schema %q reported as existing but not found", file.Schema.Name)
			}
		} else {
			return nil, fmt.Errorf("importing schema: %w", err)
		}
	} else {
		result.SchemaID = schema.ID
		result.SchemaVersion = schema.Version
		result.SchemaCreated = true
	}

	// 2. Find or create tenant.
	tenants, err := client.ListTenants(ctx, result.SchemaID)
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}

	for _, t := range tenants {
		if t.Name == file.Tenant.Name {
			result.TenantID = t.ID
			break
		}
	}

	if result.TenantID == "" {
		tenant, err := client.CreateTenant(ctx, file.Tenant.Name, result.SchemaID, result.SchemaVersion)
		if err != nil {
			return nil, fmt.Errorf("creating tenant: %w", err)
		}
		result.TenantID = tenant.ID
		result.TenantCreated = true
	}

	// 3. Import config (if values provided).
	if len(file.Config.Values) > 0 {
		configYAML, err := marshalConfigYAML(file)
		if err != nil {
			return nil, fmt.Errorf("marshaling config: %w", err)
		}

		ver, err := client.ImportConfig(ctx, result.TenantID, configYAML, file.Config.Description, adminclient.ImportModeMerge)
		if err != nil {
			return nil, fmt.Errorf("importing config: %w", err)
		}
		result.ConfigVersion = ver.Version
		result.ConfigImported = true
	}

	// 4. Apply locks.
	for _, lock := range file.Locks {
		err := client.LockField(ctx, result.TenantID, lock.FieldPath, lock.LockedValues...)
		if err != nil {
			return nil, fmt.Errorf("locking field %s: %w", lock.FieldPath, err)
		}
		result.LocksApplied++
	}

	return result, nil
}

// --- Internal helpers to marshal sub-documents ---

// marshalSchemaYAML extracts the schema section into the standard schema YAML format.
func marshalSchemaYAML(f *File) ([]byte, error) {
	doc := struct {
		SpecVersion string              `yaml:"spec_version"`
		Name        string              `yaml:"name"`
		Description string              `yaml:"description,omitempty"`
		Fields      map[string]FieldDef `yaml:"fields"`
	}{
		SpecVersion: "v1",
		Name:        f.Schema.Name,
		Description: f.Schema.Description,
		Fields:      f.Schema.Fields,
	}
	return yaml.Marshal(doc)
}

// marshalConfigYAML extracts the config section into the standard config YAML format.
func marshalConfigYAML(f *File) ([]byte, error) {
	doc := struct {
		SpecVersion string                    `yaml:"spec_version"`
		Description string                    `yaml:"description,omitempty"`
		Values      map[string]ConfigValueDef `yaml:"values"`
	}{
		SpecVersion: "v1",
		Description: f.Config.Description,
		Values:      f.Config.Values,
	}
	return yaml.Marshal(doc)
}
